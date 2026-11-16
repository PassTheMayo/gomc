package storage

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"math/bits"

	"main/src/block"
	"main/src/protocol"
	"main/src/registry"
	"main/src/world"
)

// Chunks are stored zlib compressed as:
//
//	version u8, section count VarInt, then per section the block states and
//	the biomes, each as a palette of names followed by packed indices.
//
// Storing names instead of numeric IDs keeps worlds readable after an update
// renumbers block states or biomes.
const chunkFormatVersion = 1

var biomes = registry.MustGet(registry.Biome)

func encodeChunk(c *world.Chunk) ([]byte, error) {
	w := protocol.NewWriter()
	w.UByte(chunkFormatVersion)

	c.Read(func(sections []*world.Section) {
		w.VarInt(int32(len(sections)))

		for _, s := range sections {
			encodeValues(w, s.BlockValues(), func(v int32) string { return block.StateID(v).String() })
			encodeValues(w, s.BiomeValues(), func(v int32) string { return biomes.Entries[v] })
		}
	})

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)

	if _, err := zw.Write(w.Bytes()); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// encodeValues writes the distinct values as names, then each value as a
// palette index packed into longs.
func encodeValues(w *protocol.Writer, values []int32, name func(int32) string) {
	var palette []int32
	index := make(map[int32]uint64)

	for _, v := range values {
		if _, ok := index[v]; !ok {
			index[v] = uint64(len(palette))
			palette = append(palette, v)
		}
	}

	w.VarInt(int32(len(palette)))

	for _, v := range palette {
		w.String(name(v))
	}

	width := bits.Len(uint(len(palette) - 1))
	w.UByte(uint8(width))

	if width == 0 {
		return
	}

	perLong := 64 / width
	data := make([]uint64, (len(values)+perLong-1)/perLong)

	for i, v := range values {
		data[i/perLong] |= index[v] << ((i % perLong) * width)
	}

	w.LongArray(data)
}

func decodeChunk(data []byte, pos world.ChunkPos, dim world.Dimension) (*world.Chunk, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))

	if err != nil {
		return nil, err
	}

	raw, err := io.ReadAll(zr)

	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(raw)

	if v := r.UByte(); v != chunkFormatVersion {
		return nil, fmt.Errorf("storage: unsupported chunk format version %d", v)
	}

	n := int(r.VarInt())

	if n != dim.Sections() {
		return nil, fmt.Errorf("storage: chunk has %d sections, dimension has %d", n, dim.Sections())
	}

	sections := make([]*world.Section, n)

	for i := range sections {
		blocks := decodeValues(r, 4096, func(name string) (int32, error) {
			s, err := block.Parse(name)
			return int32(s), err
		})
		biomeIDs := decodeValues(r, 64, func(name string) (int32, error) {
			id, ok := biomes.ID(name)

			if !ok {
				return 0, fmt.Errorf("storage: unknown biome %q", name)
			}

			return id, nil
		})

		if err = r.Err(); err != nil {
			return nil, err
		}

		sections[i] = world.NewSection(blocks, biomeIDs)
	}

	if err = r.Finish(); err != nil {
		return nil, err
	}

	return world.NewChunk(pos, dim, sections), nil
}

var errBadPaletteIndex = errors.New("storage: palette index out of range")

func decodeValues(r *protocol.Reader, size int, parse func(string) (int32, error)) []int32 {
	palette := make([]int32, r.Length(size))

	for i := range palette {
		v, err := parse(r.String(32767))

		if err != nil {
			r.Fail(err)
			return nil
		}

		palette[i] = v
	}

	width := int(r.UByte())
	values := make([]int32, size)

	if r.Err() != nil || len(palette) == 0 {
		r.Fail(errBadPaletteIndex)
		return nil
	}

	if width == 0 {
		for i := range values {
			values[i] = palette[0]
		}

		return values
	}

	if width > 32 {
		r.Fail(fmt.Errorf("storage: invalid palette width %d", width))
		return nil
	}

	perLong := 64 / width
	data := r.LongArray((size + perLong - 1) / perLong)
	mask := uint64(1)<<width - 1

	for i := range values {
		if i/perLong >= len(data) {
			r.Fail(errBadPaletteIndex)
			return nil
		}

		idx := data[i/perLong] >> ((i % perLong) * width) & mask

		if idx >= uint64(len(palette)) {
			r.Fail(errBadPaletteIndex)
			return nil
		}

		values[i] = palette[idx]
	}

	return values
}
