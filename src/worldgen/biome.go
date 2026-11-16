package worldgen

import "main/src/block"

// treeKind selects the shape of trees grown in a biome.
type treeKind int

const (
	treeOak treeKind = iota
	treeBirch
	treeSpruce
	treeCactus
)

// biome describes how the surface of a biome is built and decorated.
type biome struct {
	id         int32
	top        block.StateID
	filler     block.StateID
	underwater block.StateID
	depth      int
	snow       bool
	frozen     bool
	trees      []treeKind
	treeCount  int
	plants     float64
	flowers    float64
	plantKinds []block.StateID
}

func newBiome(name string, b biome) *biome {
	b.id = biomeRegistry.MustID("minecraft:" + name)

	if b.underwater == 0 {
		b.underwater = b.filler
	}

	if b.depth == 0 {
		b.depth = 3
	}

	return &b
}

var (
	biomePlains = newBiome("plains", biome{
		top: grass, filler: dirt, underwater: dirt,
		trees: []treeKind{treeOak}, treeCount: 1,
		plants: 0.3, flowers: 0.04, plantKinds: []block.StateID{shortGrass},
	})
	biomeForest = newBiome("forest", biome{
		top: grass, filler: dirt, underwater: dirt,
		trees: []treeKind{treeOak, treeOak, treeBirch}, treeCount: 9,
		plants: 0.15, flowers: 0.01, plantKinds: []block.StateID{shortGrass},
	})
	biomeBirchForest = newBiome("birch_forest", biome{
		top: grass, filler: dirt, underwater: dirt,
		trees: []treeKind{treeBirch}, treeCount: 8,
		plants: 0.15, flowers: 0.02, plantKinds: []block.StateID{shortGrass},
	})
	biomeTaiga = newBiome("taiga", biome{
		top: grass, filler: dirt, underwater: dirt,
		trees: []treeKind{treeSpruce}, treeCount: 7,
		plants: 0.2, plantKinds: []block.StateID{fern, shortGrass},
	})
	biomeSnowyTaiga = newBiome("snowy_taiga", biome{
		top: snowyGrass, filler: dirt, underwater: dirt, snow: true, frozen: true,
		trees: []treeKind{treeSpruce}, treeCount: 5,
		plants: 0.05, plantKinds: []block.StateID{fern},
	})
	biomeSnowyPlains = newBiome("snowy_plains", biome{
		top: snowyGrass, filler: dirt, underwater: dirt, snow: true, frozen: true,
		trees: []treeKind{treeSpruce}, treeCount: 1,
	})
	biomeDesert = newBiome("desert", biome{
		top: sand, filler: sand, underwater: sand, depth: 4,
		trees: []treeKind{treeCactus}, treeCount: 2,
		plants: 0.01, plantKinds: []block.StateID{deadBush},
	})
	biomeBeach      = newBiome("beach", biome{top: sand, filler: sand, depth: 4})
	biomeSnowyBeach = newBiome("snowy_beach", biome{top: sand, filler: sand, depth: 4, snow: true, frozen: true})
	biomeStonyPeaks = newBiome("stony_peaks", biome{top: stone, filler: stone})
	biomeSnowySlope = newBiome("snowy_slopes", biome{top: snowBlock, filler: snowBlock, underwater: stone, snow: true, frozen: true})
	biomeJagged     = newBiome("jagged_peaks", biome{top: snowBlock, filler: snowBlock, underwater: stone, depth: 2, frozen: true})

	biomeOcean       = newBiome("ocean", biome{top: sand, filler: sand, underwater: gravel})
	biomeDeepOcean   = newBiome("deep_ocean", biome{top: gravel, filler: gravel})
	biomeWarmOcean   = newBiome("warm_ocean", biome{top: sand, filler: sand})
	biomeLukewarm    = newBiome("lukewarm_ocean", biome{top: sand, filler: sand})
	biomeColdOcean   = newBiome("cold_ocean", biome{top: gravel, filler: gravel})
	biomeDeepCold    = newBiome("deep_cold_ocean", biome{top: gravel, filler: gravel})
	biomeFrozenOcean = newBiome("frozen_ocean", biome{top: gravel, filler: gravel, frozen: true})
	biomeDeepFrozen  = newBiome("deep_frozen_ocean", biome{top: gravel, filler: gravel, frozen: true})
)

// isOcean reports whether b is one of the ocean biomes.
func (b *biome) isOcean() bool {
	switch b {
	case biomeOcean, biomeDeepOcean, biomeWarmOcean, biomeLukewarm, biomeColdOcean, biomeDeepCold, biomeFrozenOcean, biomeDeepFrozen:
		return true
	}

	return false
}

// pickBiome chooses a biome from terrain height, temperature, humidity and how
// far inland the column is.
func pickBiome(height, seaLevel int, temperature, humidity, continental float64) *biome {
	depth := seaLevel - 1 - height

	if depth > 0 {
		deep := depth > 18

		switch {
		case temperature < -0.45:
			return pick(deep, biomeDeepFrozen, biomeFrozenOcean)
		case temperature < -0.15:
			return pick(deep, biomeDeepCold, biomeColdOcean)
		case temperature > 0.45 && !deep:
			return biomeWarmOcean
		case temperature > 0.2 && !deep:
			return biomeLukewarm
		default:
			return pick(deep, biomeDeepOcean, biomeOcean)
		}
	}

	// Mountains cool down with altitude.
	temperature -= max(0, float64(height-90)) / 100

	switch {
	case height > 175:
		return biomeJagged
	case height > 135:
		return pick(temperature < -0.3, biomeSnowySlope, biomeStonyPeaks)
	case height <= seaLevel+2 && continental < 0.08:
		return pick(temperature < -0.4, biomeSnowyBeach, biomeBeach)
	case temperature < -0.45:
		return pick(humidity > 0.1, biomeSnowyTaiga, biomeSnowyPlains)
	case temperature < -0.2:
		return biomeTaiga
	case temperature > 0.4 && humidity < 0.05:
		return biomeDesert
	case humidity > 0.35 && temperature < 0.15:
		return biomeBirchForest
	case humidity > 0.15:
		return biomeForest
	default:
		return biomePlains
	}
}

func pick(cond bool, a, b *biome) *biome {
	if cond {
		return a
	}

	return b
}
