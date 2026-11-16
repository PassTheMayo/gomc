package world

import "main/src/block"

// ChunkBuffer is a flat, mutable chunk that generators write into. Build turns
// it into a compact, immutable Chunk.
type ChunkBuffer struct {
	Pos    ChunkPos
	Dim    Dimension
	blocks []int32
	biomes []int32
}

// NewChunkBuffer returns a buffer filled with air and the given biome.
func NewChunkBuffer(pos ChunkPos, dim Dimension, biome int32) *ChunkBuffer {
	b := &ChunkBuffer{
		Pos:    pos,
		Dim:    dim,
		blocks: make([]int32, dim.Height*256),
		biomes: make([]int32, dim.Height/4*16),
	}

	for i := range b.biomes {
		b.biomes[i] = biome
	}

	return b
}

// SetBlock sets the block at chunk-local x and z and absolute y. Positions
// outside the chunk are ignored, which lets features overhang chunk borders.
func (b *ChunkBuffer) SetBlock(x, y, z int, s block.StateID) {
	y -= b.Dim.MinY

	if x < 0 || x > 15 || z < 0 || z > 15 || y < 0 || y >= b.Dim.Height {
		return
	}

	b.blocks[(y<<4|z)<<4|x] = int32(s)
}

// Block returns the block at chunk-local x and z and absolute y.
func (b *ChunkBuffer) Block(x, y, z int) block.StateID {
	y -= b.Dim.MinY

	if x < 0 || x > 15 || z < 0 || z > 15 || y < 0 || y >= b.Dim.Height {
		return 0
	}

	return block.StateID(b.blocks[(y<<4|z)<<4|x])
}

// SetBiomeColumn sets the biome of every cell in the 4x4 column containing
// chunk-local x and z.
func (b *ChunkBuffer) SetBiomeColumn(x, z int, biome int32) {
	cells := b.Dim.Height / 4

	for cy := range cells {
		b.biomes[(cy<<2|z>>2)<<2|x>>2] = biome
	}
}

// Build compacts the buffer into a chunk.
func (b *ChunkBuffer) Build() *Chunk {
	sections := make([]*Section, b.Dim.Sections())

	for i := range sections {
		sections[i] = NewSection(b.blocks[i*4096:(i+1)*4096], b.biomes[i*64:(i+1)*64])
	}

	return NewChunk(b.Pos, b.Dim, sections)
}
