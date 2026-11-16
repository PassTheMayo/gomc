package packet

import (
	"errors"

	"main/src/profile"
	"main/src/protocol"
	"main/src/text"
	"main/src/uuid"
)

// PlayerPosition teleports the player. The client must confirm it with
// AcceptTeleportation before its movement is accepted again.
type PlayerPosition struct {
	TeleportID int32
	X, Y, Z    float64
	VX, VY, VZ float64
	Yaw        float32
	Pitch      float32
	Relative   int32
}

func (p *PlayerPosition) Encode(w *protocol.Writer) {
	w.VarInt(p.TeleportID)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Double(p.VX)
	w.Double(p.VY)
	w.Double(p.VZ)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	w.Int(p.Relative)
}

func (p *PlayerPosition) Decode(r *protocol.Reader) {
	p.TeleportID = r.VarInt()
	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.VX = r.Double()
	p.VY = r.Double()
	p.VZ = r.Double()
	p.Yaw = r.Float()
	p.Pitch = r.Float()
	p.Relative = r.Int()
}

// Player info actions, combined as a bit mask in PlayerInfoUpdate.Actions.
const (
	InfoAddPlayer uint8 = 1 << iota
	InfoInitializeChat
	InfoUpdateGameMode
	InfoUpdateListed
	InfoUpdateLatency
	InfoUpdateDisplayName
	InfoUpdateListOrder
	InfoUpdateHat
)

// PlayerInfoEntry is one player in a PlayerInfoUpdate. Only the fields of the
// packet's actions are encoded.
type PlayerInfoEntry struct {
	Profile     profile.GameProfile
	GameMode    int32
	Listed      bool
	Latency     int32
	DisplayName *text.Component
	ListOrder   int32
	ShowHat     bool
}

// PlayerInfoUpdate adds players to or updates them in the tab list.
type PlayerInfoUpdate struct {
	Actions uint8
	Entries []PlayerInfoEntry
}

var errChatSession = errors.New("packet: chat session player info is not supported")

func (p *PlayerInfoUpdate) Encode(w *protocol.Writer) {
	w.UByte(p.Actions)
	protocol.WriteList(w, p.Entries, func(e PlayerInfoEntry) {
		w.UUID(e.Profile.ID)

		if p.Actions&InfoAddPlayer != 0 {
			w.String(e.Profile.Name)
			writeProperties(w, e.Profile.Properties)
		}

		if p.Actions&InfoInitializeChat != 0 {
			w.Bool(false)
		}

		if p.Actions&InfoUpdateGameMode != 0 {
			w.VarInt(e.GameMode)
		}

		if p.Actions&InfoUpdateListed != 0 {
			w.Bool(e.Listed)
		}

		if p.Actions&InfoUpdateLatency != 0 {
			w.VarInt(e.Latency)
		}

		if p.Actions&InfoUpdateDisplayName != 0 {
			w.Bool(e.DisplayName != nil)

			if e.DisplayName != nil {
				writeComponent(w, *e.DisplayName)
			}
		}

		if p.Actions&InfoUpdateListOrder != 0 {
			w.VarInt(e.ListOrder)
		}

		if p.Actions&InfoUpdateHat != 0 {
			w.Bool(e.ShowHat)
		}
	})
}

func (p *PlayerInfoUpdate) Decode(r *protocol.Reader) {
	p.Actions = r.UByte()
	p.Entries = protocol.ReadList(r, r.Remaining(), func() PlayerInfoEntry {
		e := PlayerInfoEntry{Profile: profile.GameProfile{ID: r.UUID()}}

		if p.Actions&InfoAddPlayer != 0 {
			e.Profile.Name = r.String(16)
			e.Profile.Properties = readProperties(r)
		}

		if p.Actions&InfoInitializeChat != 0 && r.Bool() {
			r.Fail(errChatSession)
		}

		if p.Actions&InfoUpdateGameMode != 0 {
			e.GameMode = r.VarInt()
		}

		if p.Actions&InfoUpdateListed != 0 {
			e.Listed = r.Bool()
		}

		if p.Actions&InfoUpdateLatency != 0 {
			e.Latency = r.VarInt()
		}

		if p.Actions&InfoUpdateDisplayName != 0 && r.Bool() {
			name := readComponent(r)
			e.DisplayName = &name
		}

		if p.Actions&InfoUpdateListOrder != 0 {
			e.ListOrder = r.VarInt()
		}

		if p.Actions&InfoUpdateHat != 0 {
			e.ShowHat = r.Bool()
		}

		return e
	})
}

// PlayerInfoRemove removes players from the tab list.
type PlayerInfoRemove struct {
	UUIDs []uuid.UUID
}

func (p *PlayerInfoRemove) Encode(w *protocol.Writer) {
	protocol.WriteList(w, p.UUIDs, w.UUID)
}

func (p *PlayerInfoRemove) Decode(r *protocol.Reader) {
	p.UUIDs = protocol.ReadList(r, r.Remaining(), r.UUID)
}

// SystemChat shows a server message in chat or, with Overlay, above the hotbar.
type SystemChat struct {
	Content text.Component
	Overlay bool
}

func (p *SystemChat) Encode(w *protocol.Writer) {
	writeComponent(w, p.Content)
	w.Bool(p.Overlay)
}

func (p *SystemChat) Decode(r *protocol.Reader) {
	p.Content = readComponent(r)
	p.Overlay = r.Bool()
}

// AcceptTeleportation confirms a PlayerPosition teleport.
type AcceptTeleportation struct {
	TeleportID int32
	X, Y, Z    float64
	Yaw        float32
	Pitch      float32
}

func (p *AcceptTeleportation) Encode(w *protocol.Writer) {
	w.VarInt(p.TeleportID)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
}

func (p *AcceptTeleportation) Decode(r *protocol.Reader) {
	p.TeleportID = r.VarInt()
	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.Yaw = r.Float()
	p.Pitch = r.Float()
}

// MoveFlags are the status flags sent with every player movement packet.
type MoveFlags struct {
	OnGround            bool
	HorizontalCollision bool
}

func (f MoveFlags) encode(w *protocol.Writer) {
	var b uint8

	if f.OnGround {
		b |= 1
	}

	if f.HorizontalCollision {
		b |= 2
	}

	w.UByte(b)
}

func (f *MoveFlags) decode(r *protocol.Reader) {
	b := r.UByte()
	f.OnGround = b&1 != 0
	f.HorizontalCollision = b&2 != 0
}

// MovePlayerPos reports a new player position.
type MovePlayerPos struct {
	X, Y, Z float64
	MoveFlags
}

func (p *MovePlayerPos) Encode(w *protocol.Writer) {
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	p.MoveFlags.encode(w)
}

func (p *MovePlayerPos) Decode(r *protocol.Reader) {
	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.MoveFlags.decode(r)
}

// MovePlayerPosRot reports a new player position and rotation.
type MovePlayerPosRot struct {
	X, Y, Z float64
	Yaw     float32
	Pitch   float32
	MoveFlags
}

func (p *MovePlayerPosRot) Encode(w *protocol.Writer) {
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	p.MoveFlags.encode(w)
}

func (p *MovePlayerPosRot) Decode(r *protocol.Reader) {
	p.X = r.Double()
	p.Y = r.Double()
	p.Z = r.Double()
	p.Yaw = r.Float()
	p.Pitch = r.Float()
	p.MoveFlags.decode(r)
}

// MovePlayerRot reports a new player rotation.
type MovePlayerRot struct {
	Yaw   float32
	Pitch float32
	MoveFlags
}

func (p *MovePlayerRot) Encode(w *protocol.Writer) {
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	p.MoveFlags.encode(w)
}

func (p *MovePlayerRot) Decode(r *protocol.Reader) {
	p.Yaw = r.Float()
	p.Pitch = r.Float()
	p.MoveFlags.decode(r)
}

// MovePlayerStatusOnly reports only the movement flags.
type MovePlayerStatusOnly struct {
	MoveFlags
}

func (p *MovePlayerStatusOnly) Encode(w *protocol.Writer) { p.MoveFlags.encode(w) }
func (p *MovePlayerStatusOnly) Decode(r *protocol.Reader) { p.MoveFlags.decode(r) }

// ChunkBatchReceived acknowledges a chunk batch and reports how many chunks
// per tick the client would like to receive.
type ChunkBatchReceived struct {
	DesiredChunksPerTick float32
}

func (p *ChunkBatchReceived) Encode(w *protocol.Writer) { w.Float(p.DesiredChunksPerTick) }
func (p *ChunkBatchReceived) Decode(r *protocol.Reader) { p.DesiredChunksPerTick = r.Float() }

// Player command actions.
const (
	CommandStartSprinting int32 = 1
	CommandStopSprinting  int32 = 2
)

// PlayerCommand reports a player action such as starting to sprint.
type PlayerCommand struct {
	EntityID int32
	Action   int32
	Data     int32
}

func (p *PlayerCommand) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.VarInt(p.Action)
	w.VarInt(p.Data)
}

func (p *PlayerCommand) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.Action = r.VarInt()
	p.Data = r.VarInt()
}

// InputSneak is set in PlayerInput.Flags while the sneak key is held.
const InputSneak uint8 = 0x20

// PlayerInput reports which movement keys are held.
type PlayerInput struct {
	Flags uint8
}

func (p *PlayerInput) Encode(w *protocol.Writer) { w.UByte(p.Flags) }
func (p *PlayerInput) Decode(r *protocol.Reader) { p.Flags = r.UByte() }

// Ability flags in PlayerAbilities and PlayerAbilitiesUpdate. Clients only
// report AbilityFlying.
const (
	AbilityInvulnerable uint8 = 0x01
	AbilityFlying       uint8 = 0x02
	AbilityCanFly       uint8 = 0x04
	AbilityInstabuild   uint8 = 0x08
)

// PlayerAbilities reports that the player started or stopped flying.
type PlayerAbilities struct {
	Flags uint8
}

func (p *PlayerAbilities) Encode(w *protocol.Writer) { w.UByte(p.Flags) }
func (p *PlayerAbilities) Decode(r *protocol.Reader) { p.Flags = r.UByte() }

// Vanilla movement speeds sent in PlayerAbilitiesUpdate.
const (
	DefaultFlyingSpeed  float32 = 0.05
	DefaultWalkingSpeed float32 = 0.1
)

// PlayerAbilitiesUpdate tells the client what the player may do and how fast
// it moves.
type PlayerAbilitiesUpdate struct {
	Flags        uint8
	FlyingSpeed  float32
	WalkingSpeed float32
}

func (p *PlayerAbilitiesUpdate) Encode(w *protocol.Writer) {
	w.UByte(p.Flags)
	w.Float(p.FlyingSpeed)
	w.Float(p.WalkingSpeed)
}

func (p *PlayerAbilitiesUpdate) Decode(r *protocol.Reader) {
	p.Flags = r.UByte()
	p.FlyingSpeed = r.Float()
	p.WalkingSpeed = r.Float()
}

// PlayerLoaded signals that the client finished loading the world.
type PlayerLoaded struct{}

func (p *PlayerLoaded) Encode(*protocol.Writer) {}
func (p *PlayerLoaded) Decode(*protocol.Reader) {}
