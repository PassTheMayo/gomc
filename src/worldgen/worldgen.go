// Package worldgen provides terrain generators.
package worldgen

import (
	"fmt"

	"main/src/block"
	"main/src/registry"
	"main/src/world"
)

// Generator names accepted by New.
const (
	NameTerrain = "terrain"
	NameFlat    = "flat"
)

// New returns the generator with the given name.
func New(name string, seed int64, dim world.Dimension) (world.Generator, error) {
	switch name {
	case NameTerrain:
		return NewTerrain(seed, dim), nil
	case NameFlat:
		return NewFlat(dim), nil
	}

	return nil, fmt.Errorf("worldgen: unknown generator %q", name)
}

// Block states placed by the generators.
var (
	air            = block.MustParse("air")
	bedrock        = block.MustParse("bedrock")
	stone          = block.MustParse("stone")
	deepslate      = block.MustParse("deepslate[axis=y]")
	dirt           = block.MustParse("dirt")
	grass          = block.MustParse("grass_block[snowy=false]")
	snowyGrass     = block.MustParse("grass_block[snowy=true]")
	sand           = block.MustParse("sand")
	sandstone      = block.MustParse("sandstone")
	gravel         = block.MustParse("gravel")
	water          = block.MustParse("water[level=0]")
	ice            = block.MustParse("ice")
	snowBlock      = block.MustParse("snow_block")
	snowLayer      = block.MustParse("snow[layers=1]")
	shortGrass     = block.MustParse("short_grass")
	fern           = block.MustParse("fern")
	deadBush       = block.MustParse("dead_bush")
	cactus         = block.MustParse("cactus[age=0]")
	dandelion      = block.MustParse("dandelion")
	poppy          = block.MustParse("poppy")
	cornflower     = block.MustParse("cornflower")
	oxeyeDaisy     = block.MustParse("oxeye_daisy")
	oakLog         = block.MustParse("oak_log[axis=y]")
	oakLeaves      = block.MustParse("oak_leaves[distance=1,persistent=true]")
	birchLog       = block.MustParse("birch_log[axis=y]")
	birchLeaves    = block.MustParse("birch_leaves[distance=1,persistent=true]")
	spruceLog      = block.MustParse("spruce_log[axis=y]")
	spruceLeaves   = block.MustParse("spruce_leaves[distance=1,persistent=true]")
	biomeRegistry  = registry.MustGet(registry.Biome)
	plainsBiomeID  = biomeRegistry.MustID("minecraft:plains")
	flowerVariants = []block.StateID{dandelion, poppy, cornflower, oxeyeDaisy}
)
