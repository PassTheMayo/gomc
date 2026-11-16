// Package noise provides seeded gradient noise for procedural generation.
package noise

import (
	"math"
	"math/rand/v2"
)

// Perlin is Ken Perlin's improved gradient noise with a seeded permutation
// table and a random offset so different instances are uncorrelated.
type Perlin struct {
	perm                      [512]uint8
	offsetX, offsetY, offsetZ float64
}

// NewPerlin returns a noise source fully determined by seed.
func NewPerlin(seed uint64) *Perlin {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	p := &Perlin{
		offsetX: rng.Float64() * 256,
		offsetY: rng.Float64() * 256,
		offsetZ: rng.Float64() * 256,
	}

	for i := range 256 {
		p.perm[i] = uint8(i)
	}

	rng.Shuffle(256, func(i, j int) { p.perm[i], p.perm[j] = p.perm[j], p.perm[i] })
	copy(p.perm[256:], p.perm[:256])

	return p
}

// Noise2 samples 2D noise in roughly [-1, 1].
func (p *Perlin) Noise2(x, y float64) float64 {
	return p.Noise3(x, y, 0)
}

// Noise3 samples 3D noise in roughly [-1, 1].
func (p *Perlin) Noise3(x, y, z float64) float64 {
	x += p.offsetX
	y += p.offsetY
	z += p.offsetZ

	fx, fy, fz := math.Floor(x), math.Floor(y), math.Floor(z)
	xi, yi, zi := int(fx)&255, int(fy)&255, int(fz)&255
	x, y, z = x-fx, y-fy, z-fz
	u, v, w := fade(x), fade(y), fade(z)

	a := int(p.perm[xi]) + yi
	aa, ab := int(p.perm[a])+zi, int(p.perm[a+1])+zi
	b := int(p.perm[xi+1]) + yi
	ba, bb := int(p.perm[b])+zi, int(p.perm[b+1])+zi

	return lerp(w,
		lerp(v,
			lerp(u, grad(p.perm[aa], x, y, z), grad(p.perm[ba], x-1, y, z)),
			lerp(u, grad(p.perm[ab], x, y-1, z), grad(p.perm[bb], x-1, y-1, z))),
		lerp(v,
			lerp(u, grad(p.perm[aa+1], x, y, z-1), grad(p.perm[ba+1], x-1, y, z-1)),
			lerp(u, grad(p.perm[ab+1], x, y-1, z-1), grad(p.perm[bb+1], x-1, y-1, z-1))))
}

func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

// grad returns the dot product of the offset with one of 12 cube edge gradients.
func grad(hash uint8, x, y, z float64) float64 {
	h := hash & 15
	u, v := x, y

	if h >= 8 {
		u = y
	}

	switch {
	case h < 4:
		v = y
	case h == 12 || h == 14:
		v = x
	default:
		v = z
	}

	if h&1 != 0 {
		u = -u
	}

	if h&2 != 0 {
		v = -v
	}

	return u + v
}
