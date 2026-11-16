package block

import "strings"

// State physics flags.
const (
	flagAir uint8 = 1 << iota
	flagSolid
	flagFluid
	flagReplaceable
)

// physics describes how a block interacts with light, movement and fluids.
type physics struct {
	opacity uint8
	flags   uint8
}

var (
	opaqueCube  = physics{opacity: 15, flags: flagSolid}
	airBlocks   = []string{"air", "cave_air", "void_air"}
	fluidBlocks = []string{"water", "lava", "bubble_column", "kelp", "kelp_plant", "seagrass", "tall_seagrass"}
	plantBlocks = []string{
		"short_grass", "tall_grass", "fern", "large_fern", "dead_bush", "dandelion", "poppy", "blue_orchid",
		"allium", "azure_bluet", "red_tulip", "orange_tulip", "white_tulip", "pink_tulip", "oxeye_daisy",
		"cornflower", "lily_of_the_valley", "sunflower", "lilac", "rose_bush", "peony", "sugar_cane",
		"oak_sapling", "spruce_sapling", "birch_sapling", "snow", "torch", "wall_torch", "vine",
		"brown_mushroom", "red_mushroom",
	}
	// replaceableBlocks give way when a block is placed into them.
	replaceableBlocks = []string{
		"short_grass", "tall_grass", "fern", "large_fern", "dead_bush", "snow", "vine", "glow_lichen",
		"seagrass", "tall_seagrass", "fire", "soul_fire", "structure_void", "light",
	}
	translucentSolids = []string{"ice", "frosted_ice", "slime_block", "honey_block"}
	clearSolids       = []string{"glass", "glass_pane", "cactus", "iron_bars", "chain"}
)

// statePhysics holds the physics of every block state.
var statePhysics []physics

// initPhysics classifies every state. Blocks without an explicit rule behave
// like opaque full cubes, which is correct for the vast majority of blocks.
func initPhysics() {
	byBlock := make([]physics, len(blocks))

	for _, b := range blocks {
		switch {
		case strings.HasSuffix(b.Name, "_leaves"):
			byBlock[b.ID] = physics{opacity: 1, flags: flagSolid}
		case strings.HasSuffix(b.Name, "_glass"), strings.HasSuffix(b.Name, "_glass_pane"):
			byBlock[b.ID] = physics{flags: flagSolid}
		default:
			byBlock[b.ID] = opaqueCube
		}
	}

	assign := func(names []string, beh physics) {
		for _, name := range names {
			if b, ok := ByName(name); ok {
				byBlock[b.ID] = beh
			}
		}
	}

	assign(airBlocks, physics{flags: flagAir | flagReplaceable})
	assign(fluidBlocks, physics{opacity: 1, flags: flagFluid})
	assign(plantBlocks, physics{})

	for _, name := range []string{"water", "lava", "bubble_column"} {
		byBlock[byName["minecraft:"+name].ID].flags |= flagReplaceable
	}

	assign(replaceableBlocks, physics{flags: flagReplaceable})
	assign(translucentSolids, physics{opacity: 1, flags: flagSolid})
	assign(clearSolids, physics{flags: flagSolid})

	statePhysics = make([]physics, len(stateOwner))

	for _, b := range blocks {
		waterlogged := hasProperty(b, "waterlogged")

		for s := b.FirstState; s < b.FirstState+StateID(b.StateCount); s++ {
			beh := byBlock[b.ID]

			if waterlogged && b.StateValues(s)["waterlogged"] == "true" {
				beh.flags |= flagFluid
			}

			statePhysics[s] = beh
		}
	}
}

func physicsOf(s StateID) physics {
	if s < 0 || int(s) >= len(statePhysics) {
		return opaqueCube
	}

	return statePhysics[s]
}

// IsAir reports whether s is one of the air blocks.
func IsAir(s StateID) bool {
	return physicsOf(s).flags&flagAir != 0
}

// LightOpacity returns how much sky light s absorbs, from 0 (none) to 15 (all).
func LightOpacity(s StateID) uint8 {
	return physicsOf(s).opacity
}

// HasFluid reports whether s contains a fluid, including waterlogged blocks.
func HasFluid(s StateID) bool {
	return physicsOf(s).flags&flagFluid != 0
}

// IsReplaceable reports whether placing a block into s replaces it, like air,
// water and grass.
func IsReplaceable(s StateID) bool {
	return physicsOf(s).flags&flagReplaceable != 0
}

// IsSolid reports whether s has a collision shape entities cannot pass.
func IsSolid(s StateID) bool {
	return physicsOf(s).flags&flagSolid != 0
}

// BlocksMotion reports whether s is counted by the MOTION_BLOCKING heightmap:
// it either has a collision shape or contains a fluid.
func BlocksMotion(s StateID) bool {
	return physicsOf(s).flags&(flagSolid|flagFluid) != 0
}
