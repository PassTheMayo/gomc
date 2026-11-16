package world

import (
	"math/bits"
	"slices"

	"main/src/block"
	"main/src/protocol"
	"main/src/registry"
)

// ContainerKind describes the palette strategy of a paletted container, which
// mirrors the vanilla client so containers serialize without conversion.
type ContainerKind struct {
	size       int
	minBits    int
	maxBits    int
	globalBits int
}

var (
	// BlockStates is the strategy of the 16x16x16 block states of a section.
	BlockStates = &ContainerKind{size: 4096, minBits: 4, maxBits: 8, globalBits: bitsFor(block.StateCount())}

	// Biomes is the strategy of the 4x4x4 biome cells of a section.
	Biomes = &ContainerKind{size: 64, minBits: 1, maxBits: 3, globalBits: bitsFor(len(registry.MustGet(registry.Biome).Entries))}
)

// bitsFor returns the number of bits needed to store n distinct values.
func bitsFor(n int) int {
	return bits.Len(uint(n - 1))
}

// Container stores values compactly as indices into a palette, or as raw
// global IDs once the palette grows too large.
type Container struct {
	kind    *ContainerKind
	bits    int
	palette []int32
	data    []uint64
}

// ContainerFromValues builds the most compact container holding values, which
// must have one entry per cell.
func ContainerFromValues(kind *ContainerKind, values []int32) *Container {
	var palette []int32
	index := make(map[int32]int32)

	// Neighbouring cells usually hold the same value, so remembering the last
	// lookup avoids most map accesses.
	lookup := func() func(int32) int32 {
		last, lastIndex := int32(-1), int32(0)

		return func(v int32) int32 {
			if v != last {
				last, lastIndex = v, index[v]
			}

			return lastIndex
		}
	}

	last := int32(-1)

	for _, v := range values {
		if v == last {
			continue
		}

		last = v

		if _, ok := index[v]; !ok {
			index[v] = int32(len(palette))
			palette = append(palette, v)
		}
	}

	c := &Container{kind: kind}
	c.bits = kind.bitsForPalette(len(palette))

	if c.bits == 0 {
		c.palette = palette
		return c
	}

	if c.bits <= kind.maxBits {
		c.palette = palette
	}

	c.data = make([]uint64, longsFor(kind.size, c.bits))
	paletteIndex := lookup()

	for i, v := range values {
		if c.palette != nil {
			v = paletteIndex(v)
		}

		c.store(i, v)
	}

	return c
}

// bitsForPalette returns the storage width for a palette of n entries.
func (k *ContainerKind) bitsForPalette(n int) int {
	if n <= 1 {
		return 0
	}

	b := max(bitsFor(n), k.minBits)

	if b > k.maxBits {
		return k.globalBits
	}

	return b
}

func longsFor(size, bits int) int {
	perLong := 64 / bits

	return (size + perLong - 1) / perLong
}

func (c *Container) store(i int, v int32) {
	perLong := 64 / c.bits
	shift := (i % perLong) * c.bits
	mask := uint64(1)<<c.bits - 1
	word := &c.data[i/perLong]

	*word = *word&^(mask<<shift) | (uint64(v)&mask)<<shift
}

func (c *Container) load(i int) int32 {
	perLong := 64 / c.bits
	shift := (i % perLong) * c.bits
	mask := uint64(1)<<c.bits - 1

	return int32(c.data[i/perLong] >> shift & mask)
}

// Get returns the value of cell i.
func (c *Container) Get(i int) int32 {
	if c.bits == 0 {
		return c.palette[0]
	}

	v := c.load(i)

	if c.palette != nil {
		return c.palette[v]
	}

	return v
}

// Set changes the value of cell i, growing the palette when needed.
func (c *Container) Set(i int, v int32) {
	switch {
	case c.bits == 0 && c.palette[0] == v:
		return
	case c.bits > 0 && c.palette == nil:
		c.store(i, v)
		return
	case c.bits > 0:
		if idx := slices.Index(c.palette, v); idx >= 0 {
			c.store(i, int32(idx))
			return
		}

		if len(c.palette) < 1<<c.bits {
			c.palette = append(c.palette, v)
			c.store(i, int32(len(c.palette)-1))

			return
		}
	}

	// The palette is full: rebuild with a wider one.
	values := c.Values()
	values[i] = v
	*c = *ContainerFromValues(c.kind, values)
}

// MayContain reports whether any cell might hold a value matching match. It
// only checks the palette, so it can report values that are no longer used.
func (c *Container) MayContain(match func(int32) bool) bool {
	if c.palette == nil {
		return true
	}

	return slices.ContainsFunc(c.palette, match)
}

// Values returns the value of every cell.
func (c *Container) Values() []int32 {
	out := make([]int32, c.kind.size)

	if c.bits == 0 {
		for i := range out {
			out[i] = c.palette[0]
		}

		return out
	}

	perLong := 64 / c.bits
	mask := uint64(1)<<c.bits - 1
	i := 0

	for _, word := range c.data {
		for j := 0; j < perLong && i < len(out); j++ {
			v := int32(word & mask)
			word >>= c.bits

			if c.palette != nil {
				v = c.palette[v]
			}

			out[i] = v
			i++
		}
	}

	return out
}

// Encode writes the container in the network format: bits per entry, the
// palette, then the packed data without a length prefix.
func (c *Container) Encode(w *protocol.Writer) {
	w.UByte(uint8(c.bits))

	switch {
	case c.bits == 0:
		w.VarInt(c.palette[0])
	case c.palette != nil:
		w.VarInt(int32(len(c.palette)))

		for _, v := range c.palette {
			w.VarInt(v)
		}
	}

	w.FixedLongArray(c.data)
}

// DecodeContainer reads a container in the network format.
func DecodeContainer(kind *ContainerKind, r *protocol.Reader) *Container {
	c := &Container{kind: kind, bits: int(r.UByte())}

	switch {
	case c.bits == 0:
		c.palette = []int32{r.VarInt()}
		return c
	case c.bits <= kind.maxBits:
		c.bits = max(c.bits, kind.minBits)
		c.palette = protocol.ReadList(r, 1<<c.bits, r.VarInt)
	}

	c.data = r.FixedLongArray(longsFor(kind.size, c.bits))

	return c
}
