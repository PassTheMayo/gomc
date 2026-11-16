package server

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"main/src/packet"
	"main/src/profile"
	"main/src/storage"
	"main/src/text"
	"main/src/world"
)

const (
	// keepAliveInterval is how often the server pings each client.
	keepAliveInterval = 15 * time.Second

	// keepAliveTimeout is how long a client may take to answer a ping.
	keepAliveTimeout = 30 * time.Second

	// maxCoordinate bounds accepted positions, like the vanilla world border.
	maxCoordinate = 3.0e7
)

var errInvalidMove = errors.New("invalid movement")

// Player is a player in the world. Fields guarded by mu are written by the
// player's session goroutine and read by the tick goroutine.
type Player struct {
	session  *session
	Profile  profile.GameProfile
	EntityID int32
	GameMode int32
	operator bool
	joined   atomic.Bool
	chunks   *chunkView

	mu               sync.Mutex
	x, y, z          float64
	yaw, pitch       float32
	onGround         bool
	sneaking         bool
	sprinting        bool
	flying           bool
	skinParts        uint8
	viewDistance     int
	latency          int32
	teleportID       int32
	awaitingTeleport bool
	keepAliveID      int64
	keepAliveSent    time.Time
	inventory        [inventorySize]packet.ItemStack
	heldSlot         int

	// Owned by the tick goroutine.
	visible   map[int32]*Player
	broadcast trackedState
}

// snapshot is a consistent copy of the state other players can observe.
type snapshot struct {
	X, Y, Z      float64
	Yaw, Pitch   float32
	OnGround     bool
	Sneaking     bool
	Sprinting    bool
	Flying       bool
	SkinParts    uint8
	ViewDistance int
	Latency      int32
}

func (p *Player) snapshot() snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	return snapshot{
		X: p.x, Y: p.y, Z: p.z, Yaw: p.yaw, Pitch: p.pitch, OnGround: p.onGround,
		Sneaking: p.sneaking, Sprinting: p.sprinting, Flying: p.flying,
		SkinParts: p.skinParts, ViewDistance: p.viewDistance, Latency: p.latency,
	}
}

// chunkPos returns the chunk the player is in.
func (s snapshot) chunkPos() world.ChunkPos {
	return world.ChunkPosAt(s.X, s.Z)
}

// savedData returns the state persisted between sessions.
func (p *Player) savedData() storage.PlayerData {
	p.mu.Lock()
	defer p.mu.Unlock()

	return storage.PlayerData{
		X: p.x, Y: p.y, Z: p.z, Yaw: p.yaw, Pitch: p.pitch,
		HeldSlot: p.heldSlot, Inventory: p.savedInventory(),
	}
}

func (p *Player) send(pk packet.Packet) {
	p.session.send(pk)
}

// Reply implements command.Source by showing the message in chat.
func (p *Player) Reply(msg text.Component) {
	p.send(&packet.SystemChat{Content: msg})
}

// IsOperator implements command.Source.
func (p *Player) IsOperator() bool {
	return p.operator
}

// teleport moves the player and ignores its movement until the client
// confirms the teleport.
func (p *Player) teleport(x, y, z float64, yaw, pitch float32) {
	p.mu.Lock()
	p.teleportID++
	id := p.teleportID
	p.awaitingTeleport = true
	p.x, p.y, p.z, p.yaw, p.pitch = x, y, z, yaw, pitch
	p.mu.Unlock()

	p.send(&packet.PlayerPosition{TeleportID: id, X: x, Y: y, Z: z, Yaw: yaw, Pitch: pitch})
}

func (p *Player) acceptTeleport(id int32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if id == p.teleportID {
		p.awaitingTeleport = false
	}
}

// move applies a movement packet. A nil position or rotation means it did
// not change. The client is authoritative over its own movement; the server
// only rejects values that cannot be valid.
func (p *Player) move(pos *[3]float64, rot *[2]float32, onGround bool) error {
	if pos != nil {
		for i, v := range pos {
			limit := maxCoordinate

			if i == 1 {
				limit = 2.0e7
			}

			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > limit {
				return errInvalidMove
			}
		}
	}

	if rot != nil && (math.IsNaN(float64(rot[0])) || math.IsInf(float64(rot[0]), 0) || math.IsNaN(float64(rot[1])) || math.IsInf(float64(rot[1]), 0)) {
		return errInvalidMove
	}

	p.mu.Lock()

	if p.awaitingTeleport {
		p.mu.Unlock()
		return nil
	}

	before := world.ChunkPosAt(p.x, p.z)

	if pos != nil {
		p.x, p.y, p.z = pos[0], pos[1], pos[2]
	}

	if rot != nil {
		p.yaw, p.pitch = rot[0], max(-90, min(rot[1], 90))
	}

	p.onGround = onGround
	after := world.ChunkPosAt(p.x, p.z)
	p.mu.Unlock()

	if after != before {
		p.chunks.moveTo(after)
	}

	return nil
}

// updateClientInformation applies changed client settings.
func (p *Player) updateClientInformation(info packet.ClientInformation, serverViewDistance int) {
	radius := max(2, min(int(info.ViewDistance), serverViewDistance))

	p.mu.Lock()
	p.skinParts = info.ModelCustomisation
	p.viewDistance = radius
	p.mu.Unlock()

	p.chunks.setRadius(radius)
}

func (p *Player) setSneaking(v bool) {
	p.mu.Lock()
	p.sneaking = v
	p.mu.Unlock()
}

func (p *Player) setSprinting(v bool) {
	p.mu.Lock()
	p.sprinting = v
	p.mu.Unlock()
}

func (p *Player) setFlying(v bool) {
	p.mu.Lock()
	p.flying = v
	p.mu.Unlock()
}

// tickKeepAlive pings the client and disconnects it if a previous ping went
// unanswered for too long.
func (p *Player) tickKeepAlive(now time.Time) {
	p.mu.Lock()
	pending, sent := p.keepAliveID != 0, p.keepAliveSent
	due := !pending && now.Sub(sent) >= keepAliveInterval

	if due {
		p.keepAliveID = now.UnixMilli()
		p.keepAliveSent = now
	}

	id := p.keepAliveID
	p.mu.Unlock()

	switch {
	case pending && now.Sub(sent) > keepAliveTimeout:
		p.session.disconnect(text.Translatable("disconnect.timeout"))
	case due:
		p.send(&packet.KeepAlive{ID: id})
	}
}

// keepAliveResponse records the round trip time of an answered ping, smoothed
// like the vanilla server does.
func (p *Player) keepAliveResponse(id int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if id != p.keepAliveID || id == 0 {
		return
	}

	rtt := int32(time.Since(p.keepAliveSent).Milliseconds())
	p.latency = (p.latency*3 + rtt) / 4
	p.keepAliveID = 0
}
