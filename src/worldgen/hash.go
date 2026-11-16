package worldgen

// hash deterministically mixes a seed with coordinates and a salt into 64
// well distributed bits, using the SplitMix64 finalizer.
func hash(seed uint64, x, z int, salt uint64) uint64 {
	h := seed ^ uint64(int64(x))*0x9E3779B97F4A7C15 ^ uint64(int64(z))*0xC2B2AE3D27D4EB4F ^ salt*0x165667B19E3779F9
	h ^= h >> 30
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 27
	h *= 0x94D049BB133111EB
	h ^= h >> 31

	return h
}

// chance returns a deterministic value in [0, 1) for a position and salt.
func chance(seed uint64, x, z int, salt uint64) float64 {
	return float64(hash(seed, x, z, salt)>>11) / (1 << 53)
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := clamp((x-edge0)/(edge1-edge0), 0, 1)

	return t * t * (3 - 2*t)
}

func clamp(v, lo, hi float64) float64 {
	return max(lo, min(v, hi))
}
