package world

import (
	"io"
	"log/slog"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"main/src/block"
	"main/src/packet"
	"main/src/protocol"
)

func TestContainerRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))

	for _, distinct := range []int{1, 2, 16, 17, 200, 300, 4096} {
		values := make([]int32, BlockStates.size)

		for i := range values {
			values[i] = int32(rng.IntN(distinct)) * 7
		}

		c := ContainerFromValues(BlockStates, values)

		if !reflect.DeepEqual(c.Values(), values) {
			t.Fatalf("%d distinct values: Values() mismatch", distinct)
		}

		w := protocol.NewWriter()
		c.Encode(w)

		r := protocol.NewReader(w.Bytes())
		decoded := DecodeContainer(BlockStates, r)

		if err := r.Finish(); err != nil {
			t.Fatalf("%d distinct values: %v", distinct, err)
		}

		if !reflect.DeepEqual(decoded.Values(), values) {
			t.Fatalf("%d distinct values: network round trip mismatch", distinct)
		}
	}
}

func TestContainerBits(t *testing.T) {
	cases := map[int]int{1: 0, 2: 4, 16: 4, 17: 5, 256: 8, 257: BlockStates.globalBits}

	for n, want := range cases {
		if got := BlockStates.bitsForPalette(n); got != want {
			t.Errorf("block palette of %d uses %d bits, want %d", n, got, want)
		}
	}

	if BlockStates.globalBits != 16 {
		t.Errorf("global block palette uses %d bits, want 16 for 35723 states", BlockStates.globalBits)
	}

	if Biomes.bitsForPalette(2) != 1 || Biomes.bitsForPalette(9) != Biomes.globalBits {
		t.Error("biome palette bits do not follow the vanilla strategy")
	}
}

func TestChunkPacket(t *testing.T) {
	stone := block.MustParse("stone")
	water := block.MustParse("water")
	buf := NewChunkBuffer(ChunkPos{X: 2, Z: -3}, Overworld, 0)

	for x := range 16 {
		for z := range 16 {
			for y := Overworld.MinY; y <= 60; y++ {
				buf.SetBlock(x, y, z, stone)
			}

			buf.SetBlock(x, 61, z, water)
			buf.SetBlock(x, 62, z, water)
		}
	}

	c := buf.Build()

	if c.Block(5, 60, 5) != stone || c.Block(5, 62, 5) != water || !block.IsAir(c.Block(5, 63, 5)) {
		t.Fatal("chunk does not hold the blocks written to the buffer")
	}

	var p *packet.LevelChunkWithLight
	c.SendPacket(func(sent *packet.LevelChunkWithLight) { p = sent })

	// Section data decodes back to the same blocks.
	r := protocol.NewReader(p.Data)

	c.Read(func(sections []*Section) {
		for i := range Overworld.Sections() {
			s := DecodeSection(r)

			if !reflect.DeepEqual(s.BlockValues(), sections[i].BlockValues()) {
				t.Fatalf("section %d does not decode to the same blocks", i)
			}
		}
	})

	if err := r.Finish(); err != nil {
		t.Fatal(err)
	}

	// Both heightmaps report the top of the water: y=62 is 62+64+1 above the bottom.
	for _, h := range p.Heightmaps {
		if got := h.Data[0] & (1<<9 - 1); got != 127 {
			t.Errorf("heightmap %d reports %d for column 0, want 127", h.Type, got)
		}
	}

	light := p.Light

	// Sections fully underground are dark, sections above the water are lit.
	if !light.EmptySkyMask.Get(1) || !light.SkyMask.Get(Overworld.Sections()+1) {
		t.Error("unexpected sky light masks")
	}

	for i := range Overworld.Sections() + 2 {
		if light.SkyMask.Get(i) == light.EmptySkyMask.Get(i) {
			t.Errorf("light section %d must be either lit or empty", i)
		}

		if !light.EmptyBlockMask.Get(i) {
			t.Errorf("light section %d must have empty block light", i)
		}
	}

	_ = packet.Encode(p)
}

type memoryStorage struct {
	saved map[ChunkPos]*Chunk
}

func (m *memoryStorage) LoadChunk(pos ChunkPos, _ Dimension) (*Chunk, error) {
	return m.saved[pos], nil
}

func (m *memoryStorage) SaveChunk(c *Chunk) error {
	m.saved[c.Pos] = c
	return nil
}

func (m *memoryStorage) HasChunk(pos ChunkPos) (bool, error) {
	_, ok := m.saved[pos]
	return ok, nil
}

type stoneGenerator struct{}

func (stoneGenerator) Generate(buf *ChunkBuffer) {
	for x := range 16 {
		for z := range 16 {
			buf.SetBlock(x, 0, z, block.MustParse("stone"))
		}
	}
}

func (stoneGenerator) Spawn() (int, int)   { return 0, 0 }
func (stoneGenerator) DefaultBiome() int32 { return 0 }

func TestWorldLifecycle(t *testing.T) {
	store := &memoryStorage{saved: make(map[ChunkPos]*Chunk)}
	w := New(Overworld, stoneGenerator{}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	pos := ChunkPos{X: 1, Z: 1}

	a := w.Acquire(pos)
	b := w.Acquire(pos)

	if a != b {
		t.Fatal("acquiring a loaded chunk must return the same chunk")
	}

	w.Release(pos)
	w.UnloadIdle(0)

	if w.LoadedChunks() != 1 {
		t.Fatal("a referenced chunk must stay loaded")
	}

	w.Release(pos)
	w.UnloadIdle(0)

	if w.LoadedChunks() != 0 || store.saved[pos] != a {
		t.Fatal("an idle chunk must be saved and unloaded")
	}

	if w.Acquire(pos) != a {
		t.Fatal("a saved chunk must be loaded from storage")
	}
}

func TestPregenerate(t *testing.T) {
	store := &memoryStorage{saved: make(map[ChunkPos]*Chunk)}
	w := New(Overworld, stoneGenerator{}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	pos := ChunkPos{X: 4, Z: -2}

	generated, err := w.Pregenerate(pos)

	if err != nil || !generated || store.saved[pos] == nil {
		t.Fatalf("Pregenerate = %v, %v; saved %v", generated, err, store.saved[pos] != nil)
	}

	if w.LoadedChunks() != 0 {
		t.Fatal("a pregenerated chunk must not stay loaded")
	}

	if generated, _ = w.Pregenerate(pos); generated {
		t.Fatal("a saved chunk must not be generated again")
	}

	viewed := ChunkPos{X: 9, Z: 9}
	w.Acquire(viewed)

	if generated, _ = w.Pregenerate(viewed); generated || store.saved[viewed] != nil {
		t.Fatal("a loaded chunk is saved when it unloads, not by pregeneration")
	}
}

func TestSetBlock(t *testing.T) {
	store := &memoryStorage{saved: make(map[ChunkPos]*Chunk)}
	w := New(Overworld, stoneGenerator{}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	stone, chest := block.MustParse("stone"), block.MustParse("chest")

	if w.SetBlock(-3, 1, 17, stone) {
		t.Fatal("blocks of unloaded chunks cannot change")
	}

	pos := ChunkPos{X: -1, Z: 1}
	c := w.Acquire(pos)

	var before *packet.LevelChunkWithLight
	c.SendPacket(func(p *packet.LevelChunkWithLight) { before = p })

	// Fill a section with many different states to force the palette to grow.
	for i := range 300 {
		w.SetBlock(-16+i%16, 100+i/256, 16+i/16%16, block.StateID(1+i))
	}

	var changes []block.StateID
	w.BlockChanged = func(x, y, z int, s block.StateID) {
		if x != -3 || y != 1 || z != 17 {
			t.Errorf("change reported at %d %d %d", x, y, z)
		}

		changes = append(changes, s)
	}

	w.SetBlock(-3, 1, 17, chest)
	w.SetBlock(-3, 1, 17, chest)

	if got, _ := w.Block(-3, 1, 17); got != chest || len(changes) != 1 || changes[0] != chest {
		t.Fatalf("block is %s with changes %v", got, changes)
	}

	if got, _ := w.Block(-16+299%16, 101, 16+299/16%16); got != block.StateID(300) {
		t.Fatalf("palette growth lost a block: %s", got)
	}

	var after *packet.LevelChunkWithLight
	c.SendPacket(func(p *packet.LevelChunkWithLight) { after = p })

	isChest := func(be packet.BlockEntity) bool {
		return be.PackedXZ == 13<<4|1 && be.Y == 1 && be.Type == blockEntityTypes.MustID("minecraft:chest")
	}

	if after == before || !slices.ContainsFunc(after.BlockEntities, isChest) {
		t.Fatalf("changed chunk packet has block entities %+v", after.BlockEntities)
	}

	// Edited chunks are saved even though they were loaded from storage.
	if err := w.Save(); err != nil || store.saved[pos] == nil {
		t.Fatalf("edited chunk was not saved: %v", err)
	}

	store.saved[pos] = nil

	if err := w.Save(); err != nil || store.saved[pos] != nil {
		t.Fatal("an unchanged chunk must not be saved again")
	}
}
