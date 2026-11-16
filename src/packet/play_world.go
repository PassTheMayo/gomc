package packet

import (
	"main/src/nbt"
	"main/src/protocol"
	"main/src/text"
)

// Game modes.
const (
	GameModeSurvival  int32 = 0
	GameModeCreative  int32 = 1
	GameModeAdventure int32 = 2
	GameModeSpectator int32 = 3
)

// GlobalPos is a block position in a specific dimension.
type GlobalPos struct {
	Dimension string
	X, Y, Z   int32
}

func (g *GlobalPos) encode(w *protocol.Writer) {
	w.String(g.Dimension)
	w.Position(g.X, g.Y, g.Z)
}

func (g *GlobalPos) decode(r *protocol.Reader) {
	g.Dimension = r.String(32767)
	g.X, g.Y, g.Z = r.Position()
}

// SpawnInfo describes the dimension a player spawns into.
type SpawnInfo struct {
	DimensionType    int32
	Dimension        string
	HashedSeed       int64
	GameMode         int32
	PreviousGameMode int32 // -1 when there is none
	Debug            bool
	Flat             bool
	DeathLocation    *GlobalPos
	PortalCooldown   int32
	SeaLevel         int32
}

func (s *SpawnInfo) encode(w *protocol.Writer) {
	w.VarInt(s.DimensionType)
	w.String(s.Dimension)
	w.Long(s.HashedSeed)
	w.VarInt(s.GameMode)
	w.VarInt(s.PreviousGameMode + 1)
	w.Bool(s.Debug)
	w.Bool(s.Flat)
	w.Bool(s.DeathLocation != nil)

	if s.DeathLocation != nil {
		s.DeathLocation.encode(w)
	}

	w.VarInt(s.PortalCooldown)
	w.VarInt(s.SeaLevel)
}

func (s *SpawnInfo) decode(r *protocol.Reader) {
	s.DimensionType = r.VarInt()
	s.Dimension = r.String(32767)
	s.HashedSeed = r.Long()
	s.GameMode = r.VarInt()
	s.PreviousGameMode = r.VarInt() - 1
	s.Debug = r.Bool()
	s.Flat = r.Bool()

	if r.Bool() {
		s.DeathLocation = &GlobalPos{}
		s.DeathLocation.decode(r)
	}

	s.PortalCooldown = r.VarInt()
	s.SeaLevel = r.VarInt()
}

// Login is the first play packet and puts the player into the world.
type Login struct {
	EntityID           int32
	Hardcore           bool
	Dimensions         []string
	MaxPlayers         int32
	ViewDistance       int32
	SimulationDistance int32
	ReducedDebugInfo   bool
	ShowDeathScreen    bool
	LimitedCrafting    bool
	Spawn              SpawnInfo
	OnlineMode         bool
	EnforcesSecureChat bool
}

func (p *Login) Encode(w *protocol.Writer) {
	w.Int(p.EntityID)
	w.Bool(p.Hardcore)
	protocol.WriteList(w, p.Dimensions, w.String)
	w.VarInt(p.MaxPlayers)
	w.VarInt(p.ViewDistance)
	w.VarInt(p.SimulationDistance)
	w.Bool(p.ReducedDebugInfo)
	w.Bool(p.ShowDeathScreen)
	w.Bool(p.LimitedCrafting)
	p.Spawn.encode(w)
	w.Bool(p.OnlineMode)
	w.Bool(p.EnforcesSecureChat)
}

func (p *Login) Decode(r *protocol.Reader) {
	p.EntityID = r.Int()
	p.Hardcore = r.Bool()
	p.Dimensions = protocol.ReadList(r, r.Remaining(), func() string { return r.String(32767) })
	p.MaxPlayers = r.VarInt()
	p.ViewDistance = r.VarInt()
	p.SimulationDistance = r.VarInt()
	p.ReducedDebugInfo = r.Bool()
	p.ShowDeathScreen = r.Bool()
	p.LimitedCrafting = r.Bool()
	p.Spawn.decode(r)
	p.OnlineMode = r.Bool()
	p.EnforcesSecureChat = r.Bool()
}

// SetDefaultSpawnPosition sets the world spawn the compass points to.
type SetDefaultSpawnPosition struct {
	Spawn GlobalPos
	Yaw   float32
	Pitch float32
}

func (p *SetDefaultSpawnPosition) Encode(w *protocol.Writer) {
	p.Spawn.encode(w)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
}

func (p *SetDefaultSpawnPosition) Decode(r *protocol.Reader) {
	p.Spawn.decode(r)
	p.Yaw = r.Float()
	p.Pitch = r.Float()
}

// GameEventLevelChunksLoadStart tells the client chunks are about to arrive so
// it can leave the loading screen once its own chunk is present.
const GameEventLevelChunksLoadStart uint8 = 13

// GameEvent signals a change in game state.
type GameEvent struct {
	Event uint8
	Value float32
}

func (p *GameEvent) Encode(w *protocol.Writer) {
	w.UByte(p.Event)
	w.Float(p.Value)
}

func (p *GameEvent) Decode(r *protocol.Reader) {
	p.Event = r.UByte()
	p.Value = r.Float()
}

// ClockState is the state of a world clock.
type ClockState struct {
	Clock       int32
	TotalTicks  int64
	PartialTick float32
	Rate        float32
}

// SetTime synchronizes the game time and world clocks.
type SetTime struct {
	GameTime int64
	Clocks   []ClockState
}

func (p *SetTime) Encode(w *protocol.Writer) {
	w.Long(p.GameTime)
	protocol.WriteList(w, p.Clocks, func(c ClockState) {
		w.VarInt(c.Clock)
		w.VarLong(c.TotalTicks)
		w.Float(c.PartialTick)
		w.Float(c.Rate)
	})
}

func (p *SetTime) Decode(r *protocol.Reader) {
	p.GameTime = r.Long()
	p.Clocks = protocol.ReadList(r, r.Remaining(), func() ClockState {
		return ClockState{Clock: r.VarInt(), TotalTicks: r.VarLong(), PartialTick: r.Float(), Rate: r.Float()}
	})
}

// Heightmap types sent to the client.
const (
	HeightmapWorldSurface   int32 = 1
	HeightmapMotionBlocking int32 = 4
)

// Heightmap is a packed array of column heights.
type Heightmap struct {
	Type int32
	Data []uint64
}

// BlockEntity is a block entity inside a chunk.
type BlockEntity struct {
	PackedXZ uint8
	Y        int16
	Type     int32
	Data     []byte // network NBT, nil when absent
}

// LightData carries the sky and block light of a chunk column. Each mask has a
// bit per light section, starting one section below the world.
type LightData struct {
	SkyMask        protocol.BitSet
	BlockMask      protocol.BitSet
	EmptySkyMask   protocol.BitSet
	EmptyBlockMask protocol.BitSet
	SkyLight       [][]byte
	BlockLight     [][]byte
}

func (l *LightData) encode(w *protocol.Writer) {
	w.BitSet(l.SkyMask)
	w.BitSet(l.BlockMask)
	w.BitSet(l.EmptySkyMask)
	w.BitSet(l.EmptyBlockMask)
	protocol.WriteList(w, l.SkyLight, w.ByteArray)
	protocol.WriteList(w, l.BlockLight, w.ByteArray)
}

func (l *LightData) decode(r *protocol.Reader) {
	l.SkyMask = r.BitSet(r.Remaining())
	l.BlockMask = r.BitSet(r.Remaining())
	l.EmptySkyMask = r.BitSet(r.Remaining())
	l.EmptyBlockMask = r.BitSet(r.Remaining())
	l.SkyLight = protocol.ReadList(r, r.Remaining(), func() []byte { return r.ByteArray(2048) })
	l.BlockLight = protocol.ReadList(r, r.Remaining(), func() []byte { return r.ByteArray(2048) })
}

// LevelChunkWithLight sends a full chunk column.
type LevelChunkWithLight struct {
	X, Z          int32
	Heightmaps    []Heightmap
	Data          []byte
	BlockEntities []BlockEntity
	Light         LightData
}

func (p *LevelChunkWithLight) Encode(w *protocol.Writer) {
	w.Int(p.X)
	w.Int(p.Z)
	protocol.WriteList(w, p.Heightmaps, func(h Heightmap) {
		w.VarInt(h.Type)
		w.LongArray(h.Data)
	})
	w.ByteArray(p.Data)
	protocol.WriteList(w, p.BlockEntities, func(be BlockEntity) {
		w.UByte(be.PackedXZ)
		w.Short(be.Y)
		w.VarInt(be.Type)

		if be.Data == nil {
			w.UByte(0)
		} else {
			w.Raw(be.Data)
		}
	})
	p.Light.encode(w)
}

func (p *LevelChunkWithLight) Decode(r *protocol.Reader) {
	p.X = r.Int()
	p.Z = r.Int()
	p.Heightmaps = protocol.ReadList(r, 16, func() Heightmap {
		return Heightmap{Type: r.VarInt(), Data: r.LongArray(r.Remaining() / 8)}
	})
	p.Data = r.ByteArray(2097152)
	p.BlockEntities = protocol.ReadList(r, r.Remaining(), func() BlockEntity {
		return BlockEntity{PackedXZ: r.UByte(), Y: r.Short(), Type: r.VarInt(), Data: readOptionalNBTBytes(r)}
	})
	p.Light.decode(r)
}

// readOptionalNBTBytes reads an optional compound tag, where a lone end tag
// means absent, and returns its raw encoding.
func readOptionalNBTBytes(r *protocol.Reader) []byte {
	start := r.Unread()

	if len(start) > 0 && start[0] == nbt.TypeEnd {
		r.Skip(1)
		return nil
	}

	readNBT(r)

	return append([]byte(nil), start[:len(start)-r.Remaining()]...)
}

// ChunkBatchStart begins a batch of chunks.
type ChunkBatchStart struct{}

func (p *ChunkBatchStart) Encode(*protocol.Writer) {}
func (p *ChunkBatchStart) Decode(*protocol.Reader) {}

// ChunkBatchFinished ends a batch of chunks, which the client acknowledges.
type ChunkBatchFinished struct {
	BatchSize int32
}

func (p *ChunkBatchFinished) Encode(w *protocol.Writer) { w.VarInt(p.BatchSize) }
func (p *ChunkBatchFinished) Decode(r *protocol.Reader) { p.BatchSize = r.VarInt() }

// ForgetLevelChunk unloads a chunk on the client.
type ForgetLevelChunk struct {
	X, Z int32
}

func (p *ForgetLevelChunk) Encode(w *protocol.Writer) { w.Long(protocol.PackChunkPos(p.X, p.Z)) }
func (p *ForgetLevelChunk) Decode(r *protocol.Reader) { p.X, p.Z = protocol.UnpackChunkPos(r.Long()) }

// SetChunkCacheCenter moves the center of the client's chunk cache.
type SetChunkCacheCenter struct {
	X, Z int32
}

func (p *SetChunkCacheCenter) Encode(w *protocol.Writer) {
	w.VarInt(p.X)
	w.VarInt(p.Z)
}

func (p *SetChunkCacheCenter) Decode(r *protocol.Reader) {
	p.X = r.VarInt()
	p.Z = r.VarInt()
}

// ServerData updates the server's description and icon in the client's
// server list while it plays.
type ServerData struct {
	Motd text.Component
	Icon []byte
}

func (p *ServerData) Encode(w *protocol.Writer) {
	writeComponent(w, p.Motd)
	w.Bool(p.Icon != nil)

	if p.Icon != nil {
		w.ByteArray(p.Icon)
	}
}

func (p *ServerData) Decode(r *protocol.Reader) {
	p.Motd = readComponent(r)

	if r.Bool() {
		p.Icon = r.ByteArray(r.Remaining())
	}
}
