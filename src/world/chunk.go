package world

import (
	"math"
	"sync"

	"main/src/block"
	"main/src/packet"
)

// ChunkPos identifies a chunk column.
type ChunkPos struct {
	X, Z int32
}

// ChunkPosAt returns the chunk containing the block column at x, z.
func ChunkPosAt(x, z float64) ChunkPos {
	return ChunkPos{X: int32(math.Floor(x)) >> 4, Z: int32(math.Floor(z)) >> 4}
}

// Chunk is a chunk column. Its blocks can change while players view and save
// it, so every access goes through its lock.
type Chunk struct {
	Pos ChunkPos
	dim Dimension

	mu       sync.Mutex
	sections []*Section
	version  uint64
	packet   *packet.LevelChunkWithLight
}

// NewChunk assembles a chunk from its sections, ordered bottom to top.
func NewChunk(pos ChunkPos, dim Dimension, sections []*Section) *Chunk {
	return &Chunk{Pos: pos, dim: dim, sections: sections}
}

// Read calls fn with the sections of the chunk, ordered bottom to top, while
// holding its lock. fn must not keep the sections.
func (c *Chunk) Read(fn func(sections []*Section)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fn(c.sections)
}

// Version returns a counter that increases with every block change.
func (c *Chunk) Version() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.version
}

// Block returns the block state at chunk-local x and z and absolute y.
func (c *Chunk) Block(x, y, z int) block.StateID {
	c.mu.Lock()
	defer c.mu.Unlock()

	if y < c.dim.MinY || y > c.dim.MaxY() {
		return block.StateID(0)
	}

	y -= c.dim.MinY

	return c.sections[y>>4].Block(x, y&15, z)
}

// setBlock changes a block and, if it changed, calls changed while still
// holding the lock so that anything it sends is ordered with chunk packets.
func (c *Chunk) setBlock(x, y, z int, s block.StateID, changed func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if y < c.dim.MinY || y > c.dim.MaxY() {
		return
	}

	ly := y - c.dim.MinY

	if c.sections[ly>>4].setBlock(x, ly&15, z, s) == s {
		return
	}

	c.version++
	c.packet = nil
	changed()
}

// SendPacket calls send with the network representation of the chunk while
// holding its lock. The packet is built when first needed after a change and
// shared by every player the chunk is sent to. Holding the lock guarantees a
// player never receives a chunk older than a block update already sent.
func (c *Chunk) SendPacket(send func(*packet.LevelChunkWithLight)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.packet == nil {
		c.packet = c.buildPacket()
	}

	send(c.packet)
}
