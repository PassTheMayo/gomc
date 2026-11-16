package world

import (
	"iter"
	"slices"
	"sync"
)

// InView reports whether pos is within radius chunks of center, using the
// rounded square of the vanilla server.
func InView(center ChunkPos, radius int, pos ChunkPos) bool {
	dx := max(0, distance(pos.X, center.X)-1)
	dz := max(0, distance(pos.Z, center.Z)-1)

	return int(dx*dx+dz*dz) < radius*radius
}

func distance(a, b int32) int32 {
	if a < b {
		return b - a
	}

	return a - b
}

var (
	viewOffsetsMu sync.Mutex
	viewOffsets   = make(map[int][]ChunkPos)
)

// InViewNearestFirst yields every chunk InView of center within radius,
// nearest first. The offsets for each radius are sorted once and reused.
func InViewNearestFirst(center ChunkPos, radius int) iter.Seq[ChunkPos] {
	offsets := offsetsInView(radius)

	return func(yield func(ChunkPos) bool) {
		for _, o := range offsets {
			if !yield(ChunkPos{X: center.X + o.X, Z: center.Z + o.Z}) {
				return
			}
		}
	}
}

func offsetsInView(radius int) []ChunkPos {
	viewOffsetsMu.Lock()
	defer viewOffsetsMu.Unlock()

	if offsets, ok := viewOffsets[radius]; ok {
		return offsets
	}

	var offsets []ChunkPos
	r := int32(radius + 1)

	for x := -r; x <= r; x++ {
		for z := -r; z <= r; z++ {
			if o := (ChunkPos{X: x, Z: z}); InView(ChunkPos{}, radius, o) {
				offsets = append(offsets, o)
			}
		}
	}

	slices.SortStableFunc(offsets, func(a, b ChunkPos) int {
		return int(a.X*a.X+a.Z*a.Z) - int(b.X*b.X+b.Z*b.Z)
	})

	viewOffsets[radius] = offsets

	return offsets
}

// Ring returns the chunks exactly r chunks from center along either axis, the
// outline of a square.
func Ring(center ChunkPos, r int32) []ChunkPos {
	if r == 0 {
		return []ChunkPos{center}
	}

	out := make([]ChunkPos, 0, 8*r)

	for d := -r; d <= r; d++ {
		out = append(out, ChunkPos{X: center.X + d, Z: center.Z - r}, ChunkPos{X: center.X + d, Z: center.Z + r})
	}

	for d := -r + 1; d <= r-1; d++ {
		out = append(out, ChunkPos{X: center.X - r, Z: center.Z + d}, ChunkPos{X: center.X + r, Z: center.Z + d})
	}

	return out
}
