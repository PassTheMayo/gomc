package world

import (
	"main/src/block"
	"main/src/protocol"
)

// Section is a 16x16x16 cube of blocks with its 4x4x4 biome cells.
type Section struct {
	blocks *Container
	biomes *Container
	nonAir int
	fluids int
}

// NewSection builds a section from one block state per block, indexed
// (y*16+z)*16+x, and one biome ID per cell, indexed (y*4+z)*4+x.
func NewSection(blocks, biomes []int32) *Section {
	s := &Section{
		blocks: ContainerFromValues(BlockStates, blocks),
		biomes: ContainerFromValues(Biomes, biomes),
	}

	last, air, fluid := int32(-1), false, false

	for _, b := range blocks {
		if b != last {
			last = b
			air, fluid = block.IsAir(block.StateID(b)), block.HasFluid(block.StateID(b))
		}

		if !air {
			s.nonAir++
		}

		if fluid {
			s.fluids++
		}
	}

	return s
}

// Block returns the block state at local coordinates.
func (s *Section) Block(x, y, z int) block.StateID {
	return block.StateID(s.blocks.Get(y<<8 | z<<4 | x))
}

// setBlock changes a block and returns the state it replaced.
func (s *Section) setBlock(x, y, z int, state block.StateID) block.StateID {
	i := y<<8 | z<<4 | x
	old := block.StateID(s.blocks.Get(i))

	if old == state {
		return old
	}

	s.blocks.Set(i, int32(state))
	s.nonAir += boolDelta(!block.IsAir(state)) - boolDelta(!block.IsAir(old))
	s.fluids += boolDelta(block.HasFluid(state)) - boolDelta(block.HasFluid(old))

	return old
}

func boolDelta(v bool) int {
	if v {
		return 1
	}

	return 0
}

// BlockValues returns every block state in the section.
func (s *Section) BlockValues() []int32 {
	return s.blocks.Values()
}

// BiomeValues returns every biome ID in the section.
func (s *Section) BiomeValues() []int32 {
	return s.biomes.Values()
}

// IsEmpty reports whether the section contains only air.
func (s *Section) IsEmpty() bool {
	return s.nonAir == 0
}

// Encode writes the section in the network format.
func (s *Section) Encode(w *protocol.Writer) {
	w.Short(int16(s.nonAir))
	w.Short(int16(s.fluids))
	s.blocks.Encode(w)
	s.biomes.Encode(w)
}

// DecodeSection reads a section in the network format.
func DecodeSection(r *protocol.Reader) *Section {
	s := &Section{nonAir: int(r.Short()), fluids: int(r.Short())}
	s.blocks = DecodeContainer(BlockStates, r)
	s.biomes = DecodeContainer(Biomes, r)

	return s
}
