package worldgen

import (
	"main/src/block"
	"main/src/world"
)

// tree is a tree or cactus rooted at a world position.
type tree struct {
	x, y, z int
	kind    treeKind
	height  int
	shape   uint64
}

// treesIn returns the trees rooted in a chunk. Every chunk recomputes the trees
// of its neighbours too, so canopies crossing chunk borders line up.
func (t *Terrain) treesIn(chunkX, chunkZ int) []tree {
	center := t.column(chunkX*16+8, chunkZ*16+8)
	attempts := center.biome.treeCount

	if attempts == 0 {
		return nil
	}

	var trees []tree

	for i := range attempts {
		h := hash(t.seed, chunkX, chunkZ, saltTree+uint64(i)<<8)
		wx, wz := chunkX*16+int(h&15), chunkZ*16+int(h>>4&15)
		col := t.column(wx, wz)
		b := col.biome

		if len(b.trees) == 0 || col.height < t.dim.SeaLevel {
			continue
		}

		kind := b.trees[(h>>8)%uint64(len(b.trees))]

		if (kind == treeCactus) != (b.top == sand) {
			continue
		}

		// Sparse biomes skip most of their single attempt.
		if attempts == 1 && h>>40&3 != 0 {
			continue
		}

		tr := tree{x: wx, y: col.height + 1, z: wz, kind: kind, shape: h}

		switch kind {
		case treeOak:
			tr.height = 4 + int(h>>16%3)
		case treeBirch:
			tr.height = 5 + int(h>>16%3)
		case treeSpruce:
			tr.height = 6 + int(h>>16%4)
		case treeCactus:
			tr.height = 1 + int(h>>16%3)
		}

		trees = append(trees, tr)
	}

	return trees
}

// grow places the parts of the tree that fall inside the buffer's chunk.
func (tr tree) grow(buf *world.ChunkBuffer, originX, originZ int) {
	set := func(x, y, z int, s block.StateID, overwriteLeaves bool) {
		lx, lz := x-originX, z-originZ
		cur := buf.Block(lx, y, lz)

		if cur == air || cur == shortGrass || cur == fern || cur == snowLayer || (overwriteLeaves && isLeaves(cur)) {
			buf.SetBlock(lx, y, lz, s)
		}
	}

	switch tr.kind {
	case treeCactus:
		for i := range tr.height {
			set(tr.x, tr.y+i, tr.z, cactus, false)
		}
	case treeOak, treeBirch:
		log, leaves := oakLog, oakLeaves

		if tr.kind == treeBirch {
			log, leaves = birchLog, birchLeaves
		}

		top := tr.y + tr.height - 1

		for y := top - 2; y <= top+1; y++ {
			radius := 2

			if y >= top {
				radius = 1
			}

			for dx := -radius; dx <= radius; dx++ {
				for dz := -radius; dz <= radius; dz++ {
					corner := abs(dx) == radius && abs(dz) == radius

					// Corners of the wide layers are trimmed at random, and the
					// top layer is a plus shape.
					if corner && (radius == 1 || tr.shape>>(uint(y-top+2)*4+uint(dx+2)+uint(dz+2))&1 == 0) {
						continue
					}

					set(tr.x+dx, y, tr.z+dz, leaves, false)
				}
			}
		}

		for i := range tr.height {
			set(tr.x, tr.y+i, tr.z, log, true)
		}
	case treeSpruce:
		top := tr.y + tr.height

		set(tr.x, top, tr.z, spruceLeaves, false)

		for y := top - 1; y >= tr.y+2; y-- {
			step := top - y
			radius := min(3, (step+1)/2)

			if step%2 == 0 {
				radius--
			}

			for dx := -radius; dx <= radius; dx++ {
				for dz := -radius; dz <= radius; dz++ {
					if radius > 0 && abs(dx) == radius && abs(dz) == radius {
						continue
					}

					set(tr.x+dx, y, tr.z+dz, spruceLeaves, false)
				}
			}
		}

		for i := range tr.height {
			set(tr.x, tr.y+i, tr.z, spruceLog, true)
		}
	}
}

func isLeaves(s block.StateID) bool {
	return s == oakLeaves || s == birchLeaves || s == spruceLeaves
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
