package worldgen

import (
	"reflect"
	"testing"

	"main/src/block"
	"main/src/world"
)

func generate(g world.Generator, x, z int32) *world.Chunk {
	buf := world.NewChunkBuffer(world.ChunkPos{X: x, Z: z}, world.Overworld, g.DefaultBiome())
	g.Generate(buf)

	return buf.Build()
}

func TestTerrainIsDeterministic(t *testing.T) {
	a := generate(NewTerrain(42, world.Overworld), 3, -7)
	b := generate(NewTerrain(42, world.Overworld), 3, -7)
	c := generate(NewTerrain(43, world.Overworld), 3, -7)

	blocks := func(c *world.Chunk) (out [][]int32) {
		c.Read(func(sections []*world.Section) {
			for _, s := range sections {
				out = append(out, s.BlockValues())
			}
		})

		return out
	}

	same := func(x, y *world.Chunk) bool { return reflect.DeepEqual(blocks(x), blocks(y)) }

	if !same(a, b) {
		t.Fatal("the same seed must generate the same chunk")
	}

	if same(a, c) {
		t.Fatal("different seeds should generate different chunks")
	}
}

func TestTerrainShape(t *testing.T) {
	g := NewTerrain(12345, world.Overworld)
	land, sea, total := 0, 0, 0

	for x := -3000; x < 3000; x += 97 {
		for z := -3000; z < 3000; z += 97 {
			h := g.column(x, z).height
			total++

			if h >= world.Overworld.SeaLevel {
				land++
			} else {
				sea++
			}

			if h <= world.Overworld.MinY || h >= world.Overworld.MaxY() {
				t.Fatalf("height %d at %d,%d is outside the world", h, x, z)
			}
		}
	}

	if land < total/5 || sea < total/10 {
		t.Fatalf("expected a mix of land and sea, got %d land and %d sea of %d", land, sea, total)
	}
}

func TestTerrainColumns(t *testing.T) {
	g := NewTerrain(7, world.Overworld)
	x, z := g.Spawn()
	c := generate(g, int32(x>>4), int32(z>>4))
	lx, lz := x&15, z&15

	if c.Block(lx, world.Overworld.MinY, lz) != bedrock {
		t.Error("the bottom of the world must be bedrock")
	}

	top := g.column(x, z).height

	if top < world.Overworld.SeaLevel {
		t.Fatalf("spawn column at height %d is under water", top)
	}

	if !block.BlocksMotion(c.Block(lx, top, lz)) {
		t.Errorf("spawn column top %s is not solid", c.Block(lx, top, lz))
	}
}

func TestFlat(t *testing.T) {
	c := generate(NewFlat(world.Overworld), 0, 0)

	want := []block.StateID{bedrock, dirt, dirt, grass}

	for i, s := range want {
		if got := c.Block(3, world.Overworld.MinY+i, 3); got != s {
			t.Errorf("layer %d is %s, want %s", i, got, s)
		}
	}

	if !block.IsAir(c.Block(3, world.Overworld.MinY+len(want), 3)) {
		t.Error("everything above the layers must be air")
	}
}
