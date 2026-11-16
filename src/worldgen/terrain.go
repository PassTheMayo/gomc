package worldgen

import (
	"math"

	"main/src/block"
	"main/src/noise"
	"main/src/world"
)

// Salts that decorrelate the random decisions made for the same column.
const (
	saltBedrock uint64 = iota + 1
	saltDeepslate
	saltPlant
	saltPlantKind
	saltTree
)

// Terrain generates continents, oceans, hills and mountains from layered
// noise, with biomes picked by height, temperature and humidity.
type Terrain struct {
	seed uint64
	dim  world.Dimension

	continental *noise.Fractal
	erosion     *noise.Fractal
	hills       *noise.Fractal
	detail      *noise.Fractal
	ridges      *noise.Fractal
	temperature *noise.Fractal
	humidity    *noise.Fractal
}

// NewTerrain returns a terrain generator for seed.
func NewTerrain(seed int64, dim world.Dimension) *Terrain {
	s := uint64(seed)

	return &Terrain{
		seed:        s,
		dim:         dim,
		continental: noise.NewFractal(s+1, 6, 1.0/1400, 0.5),
		erosion:     noise.NewFractal(s+2, 4, 1.0/700, 0.5),
		hills:       noise.NewFractal(s+3, 5, 1.0/220, 0.5),
		detail:      noise.NewFractal(s+4, 3, 1.0/40, 0.5),
		ridges:      noise.NewFractal(s+5, 5, 1.0/500, 0.45),
		temperature: noise.NewFractal(s+6, 3, 1.0/1100, 0.5),
		humidity:    noise.NewFractal(s+7, 3, 1.0/900, 0.5),
	}
}

// column is the terrain height and biome of one block column.
type column struct {
	height int
	biome  *biome
}

// continentSpline maps continentalness to a base height, from deep ocean
// floors through the shoreline to inland plateaus.
var continentSpline = [][2]float64{
	{-1, 28}, {-0.5, 38}, {-0.25, 50}, {-0.1, 58}, {0, 62}, {0.08, 65}, {0.3, 70}, {0.6, 80}, {1, 95},
}

func spline(points [][2]float64, x float64) float64 {
	if x <= points[0][0] {
		return points[0][1]
	}

	for i := 1; i < len(points); i++ {
		if x <= points[i][0] {
			a, b := points[i-1], points[i]

			return a[1] + (x-a[0])/(b[0]-a[0])*(b[1]-a[1])
		}
	}

	return points[len(points)-1][1]
}

// column computes the height and biome of the column at world x, z. It only
// depends on the seed, so neighbouring chunks agree on shared features.
func (t *Terrain) column(x, z int) column {
	fx, fz := float64(x), float64(z)

	c := clamp(t.continental.Sample2(fx, fz)*2.4+0.15, -1, 1)
	e := clamp(t.erosion.Sample2(fx, fz)*2.2, -1, 1)
	land := smoothstep(-0.25, 0.1, c)

	h := spline(continentSpline, c)
	h += t.hills.Sample2(fx, fz) * 2 * (3 + land*(6+16*smoothstep(-0.4, 0.6, e)))
	h += t.detail.Sample2(fx, fz) * 2.5 * land

	if mountain := smoothstep(0.15, 0.55, e) * smoothstep(0.05, 0.4, c); mountain > 0 {
		ridge := clamp(1-math.Abs(t.ridges.Sample2(fx, fz)*2.2), 0, 1)
		h += (ridge*ridge*ridge*120 + 20) * mountain
	}

	height := int(math.Round(clamp(h, float64(t.dim.MinY+8), float64(t.dim.MaxY()-40))))
	temperature := clamp(t.temperature.Sample2(fx, fz)*2.5, -1, 1)
	humidity := clamp(t.humidity.Sample2(fx, fz)*2.5, -1, 1)

	return column{height: height, biome: pickBiome(height, t.dim.SeaLevel, temperature, humidity, c)}
}

// Generate implements world.Generator.
func (t *Terrain) Generate(buf *world.ChunkBuffer) {
	originX, originZ := int(buf.Pos.X)*16, int(buf.Pos.Z)*16

	var cols [16][16]column

	for x := range 16 {
		for z := range 16 {
			cols[x][z] = t.column(originX+x, originZ+z)
		}
	}

	for x := 0; x < 16; x += 4 {
		for z := 0; z < 16; z += 4 {
			buf.SetBiomeColumn(x, z, cols[x+2][z+2].biome.id)
		}
	}

	for x := range 16 {
		for z := range 16 {
			t.fillColumn(buf, x, z, originX+x, originZ+z, cols[x][z])
			t.decorateColumn(buf, x, z, originX+x, originZ+z, cols[x][z])
		}
	}

	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			for _, tr := range t.treesIn(int(buf.Pos.X)+dx, int(buf.Pos.Z)+dz) {
				tr.grow(buf, originX, originZ)
			}
		}
	}
}

// fillColumn places bedrock, stone, deepslate, the biome surface and water.
func (t *Terrain) fillColumn(buf *world.ChunkBuffer, x, z, wx, wz int, col column) {
	minY, seaLevel := t.dim.MinY, t.dim.SeaLevel
	b := col.biome
	submerged := col.height < seaLevel-1
	sandy := b.top == sand

	for y := minY; y <= col.height; y++ {
		var s block.StateID
		below := col.height - y

		switch {
		case y == minY:
			s = bedrock
		case y <= minY+4 && chance(t.seed, wx, wz, saltBedrock+uint64(y-minY)<<8) < float64(minY+5-y)/5:
			s = bedrock
		case below < b.depth:
			s = b.filler

			if below == 0 && !submerged {
				s = b.top
			} else if submerged {
				s = b.underwater
			}
		case sandy && below < b.depth+3:
			s = sandstone
		case y < 0 || (y < 8 && chance(t.seed, wx, wz, saltDeepslate+uint64(y)<<8) < float64(8-y)/8):
			s = deepslate
		default:
			s = stone
		}

		buf.SetBlock(x, y, z, s)
	}

	if submerged {
		for y := col.height + 1; y < seaLevel; y++ {
			buf.SetBlock(x, y, z, water)
		}

		if b.frozen {
			buf.SetBlock(x, seaLevel-1, z, ice)
		}
	} else if b.snow && b.top != snowBlock {
		buf.SetBlock(x, col.height+1, z, snowLayer)
	}
}

// decorateColumn scatters grass, ferns, flowers and dead bushes.
func (t *Terrain) decorateColumn(buf *world.ChunkBuffer, x, z, wx, wz int, col column) {
	b := col.biome
	y := col.height + 1

	if b.snow || col.height < t.dim.SeaLevel-1 || buf.Block(x, y, z) != air {
		return
	}

	roll := chance(t.seed, wx, wz, saltPlant)
	pick := hash(t.seed, wx, wz, saltPlantKind)
	ground := buf.Block(x, col.height, z)

	switch {
	case roll < b.flowers && ground == grass:
		buf.SetBlock(x, y, z, flowerVariants[pick%uint64(len(flowerVariants))])
	case roll < b.flowers+b.plants && len(b.plantKinds) > 0 && (ground == grass || ground == sand):
		buf.SetBlock(x, y, z, b.plantKinds[pick%uint64(len(b.plantKinds))])
	}
}

// Spawn implements world.Generator. It searches outwards from the origin for
// dry land that is not a mountain peak.
func (t *Terrain) Spawn() (x, z int) {
	for r := 0; r < 4096; r += 16 {
		for i := -r; i <= r; i += 16 {
			for _, p := range [][2]int{{i, -r}, {i, r}, {-r, i}, {r, i}} {
				col := t.column(p[0], p[1])

				if col.height >= t.dim.SeaLevel && !col.biome.isOcean() && col.height < 120 {
					return p[0], p[1]
				}
			}
		}
	}

	return 0, 0
}

// DefaultBiome implements world.Generator.
func (t *Terrain) DefaultBiome() int32 {
	return plainsBiomeID
}
