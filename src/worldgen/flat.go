package worldgen

import (
	"main/src/block"
	"main/src/world"
)

// Flat generates the classic superflat layers: bedrock, two dirt and grass.
type Flat struct {
	dim    world.Dimension
	layers []block.StateID
}

// NewFlat returns a superflat generator.
func NewFlat(dim world.Dimension) *Flat {
	return &Flat{dim: dim, layers: []block.StateID{bedrock, dirt, dirt, grass}}
}

// Generate implements world.Generator.
func (f *Flat) Generate(buf *world.ChunkBuffer) {
	for x := range 16 {
		for z := range 16 {
			for i, s := range f.layers {
				buf.SetBlock(x, f.dim.MinY+i, z, s)
			}
		}
	}
}

// Spawn implements world.Generator.
func (f *Flat) Spawn() (x, z int) {
	return 0, 0
}

// DefaultBiome implements world.Generator.
func (f *Flat) DefaultBiome() int32 {
	return plainsBiomeID
}
