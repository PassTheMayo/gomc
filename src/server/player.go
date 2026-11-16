package server

import (
	"sync"
	"sync/atomic"
	"time"

	"main/src/entity"
	"main/src/inventory"
	"main/src/packet"
	"main/src/profile"
	"main/src/storage"
	"main/src/text"
	"main/src/world"
)

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
	inventory        *inventory.Player
	swing            entity.Swing
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
		HeldSlot: p.inventory.Selected(), Inventory: p.inventory.Save(),
	}
}

func (p *Player) Send(pk packet.Packet) {
	p.session.send(pk)
}

// Reply implements command.Source by showing the message in chat.
func (p *Player) Reply(msg text.Component) {
	p.Send(&packet.SystemChat{Content: msg})
}

// IsOperator implements command.Source.
func (p *Player) IsOperator() bool {
	return p.operator
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

// registerClientInformation applies the settings clients send when they
// change them in play.
func (s *Server) registerClientInformation() {
	on(&s.router, func(p *Player, pk *packet.ClientInformation) {
		p.updateClientInformation(*pk, s.conf.ViewDistance)
	})
}
