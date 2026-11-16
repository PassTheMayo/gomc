package noise

// Fractal sums several octaves of Perlin noise, each at double the frequency
// and a fraction of the amplitude of the previous one.
type Fractal struct {
	octaves     []*Perlin
	frequency   float64
	persistence float64
	norm        float64
}

// NewFractal returns fractal noise with the given number of octaves. Frequency
// is the scale of the first octave and persistence the amplitude ratio between
// consecutive octaves.
func NewFractal(seed uint64, octaves int, frequency, persistence float64) *Fractal {
	f := &Fractal{frequency: frequency, persistence: persistence}
	amplitude := 1.0

	for i := range octaves {
		f.octaves = append(f.octaves, NewPerlin(seed+uint64(i)*0x632BE59BD9B4E019))
		f.norm += amplitude
		amplitude *= persistence
	}

	return f
}

// Sample2 returns 2D fractal noise in roughly [-1, 1].
func (f *Fractal) Sample2(x, z float64) float64 {
	sum, amplitude, frequency := 0.0, 1.0, f.frequency

	for _, o := range f.octaves {
		sum += o.Noise2(x*frequency, z*frequency) * amplitude
		amplitude *= f.persistence
		frequency *= 2
	}

	return sum / f.norm
}
