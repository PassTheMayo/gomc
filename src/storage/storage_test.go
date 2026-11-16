package storage

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"main/src/block"
	"main/src/protocol"
	"main/src/uuid"
	"main/src/version"
	"main/src/world"
)

func testChunk(pos world.ChunkPos) *world.Chunk {
	buf := world.NewChunkBuffer(pos, world.Overworld, biomes.MustID("minecraft:plains"))
	oak := block.MustParse("oak_log[axis=x]")

	for x := range 16 {
		for z := range 16 {
			buf.SetBlock(x, -64, z, block.MustParse("bedrock"))
			buf.SetBlock(x, x+z, z, oak)
		}
	}

	buf.SetBiomeColumn(0, 0, biomes.MustID("minecraft:desert"))

	return buf.Build()
}

func chunkValues(c *world.Chunk) (out [][]int32) {
	c.Read(func(sections []*world.Section) {
		for _, s := range sections {
			out = append(out, s.BlockValues(), s.BiomeValues())
		}
	})

	return out
}

func sameChunk(t *testing.T, a, b *world.Chunk) {
	t.Helper()

	if !reflect.DeepEqual(chunkValues(a), chunkValues(b)) {
		t.Fatal("chunks differ")
	}
}

func TestChunkRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)

	if err != nil {
		t.Fatal(err)
	}

	positions := []world.ChunkPos{{X: 0, Z: 0}, {X: -1, Z: 33}, {X: 31, Z: 31}}

	for _, pos := range positions {
		if err = store.SaveChunk(testChunk(pos)); err != nil {
			t.Fatal(err)
		}
	}

	missing, err := store.LoadChunk(world.ChunkPos{X: 5, Z: 5}, world.Overworld)

	if err != nil || missing != nil {
		t.Fatalf("unsaved chunk loaded as %v, %v", missing, err)
	}

	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen to make sure everything was persisted.
	store, err = Open(dir)

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	for _, pos := range positions {
		c, err := store.LoadChunk(pos, world.Overworld)

		if err != nil || c == nil {
			t.Fatalf("loading %v: %v", pos, err)
		}

		sameChunk(t, c, testChunk(pos))
	}
}

func TestCorruptRecord(t *testing.T) {
	dir := t.TempDir()
	store, _ := Open(dir)
	pos := world.ChunkPos{X: 0, Z: 0}

	if err := store.SaveChunk(testChunk(pos)); err != nil {
		t.Fatal(err)
	}

	store.Close()

	path := filepath.Join(dir, "regions", "r.0.0.region")
	data, _ := os.ReadFile(path)
	data[len(data)-10] ^= 0xFF
	_ = os.WriteFile(path, data, 0o644)

	store, _ = Open(dir)
	defer store.Close()

	if _, err := store.LoadChunk(pos, world.Overworld); err == nil {
		t.Fatal("expected a checksum error for a corrupt record")
	}
}

func TestPlayerAndLevelData(t *testing.T) {
	store, err := Open(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	id := uuid.New()

	if _, found, err := store.LoadPlayer(id); found || err != nil {
		t.Fatalf("unknown player: found=%v err=%v", found, err)
	}

	p := PlayerData{X: 1.5, Y: 70, Z: -8.25, Yaw: 90, Pitch: -10, HeldSlot: 3, Inventory: []InventoryItem{
		{Slot: 36, Item: "minecraft:stone", Count: 64},
		{Slot: 45, Item: "minecraft:diamond_sword", Count: 1, Components: []ItemComponent{{Type: 13, Data: []byte{1, 2, 3}}}, Removed: []int32{7}},
	}}

	if err = store.SavePlayer(id, p); err != nil {
		t.Fatal(err)
	}

	if got, found, err := store.LoadPlayer(id); !found || err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("LoadPlayer = %+v, %v, %v", got, found, err)
	}

	l := LevelData{Seed: -1234567890123, GameTime: 24000, Generator: "terrain"}

	if err = store.SaveLevel(l); err != nil {
		t.Fatal(err)
	}

	if got, found, err := store.LoadLevel(); !found || err != nil || got != l {
		t.Fatalf("LoadLevel = %+v, %v, %v", got, found, err)
	}
}

// Version 1 files, written before inventories were saved, still load.
func TestPlayerDataVersion1(t *testing.T) {
	store, err := Open(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	id := uuid.New()
	w := protocol.NewWriter()
	w.Raw([]byte(playerMagic))
	w.UByte(1)
	w.Double(1.5)
	w.Double(70)
	w.Double(-8.25)
	w.Float(90)
	w.Float(-10)

	if err = writeFileAtomic(store.playerPath(id), w.Bytes()); err != nil {
		t.Fatal(err)
	}

	want := PlayerData{X: 1.5, Y: 70, Z: -8.25, Yaw: 90, Pitch: -10}

	if got, found, err := store.LoadPlayer(id); !found || err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadPlayer = %+v, %v, %v", got, found, err)
	}
}

// Item components saved by another protocol version may hold different IDs,
// so they are dropped while the items themselves are kept.
func TestPlayerDataOtherProtocol(t *testing.T) {
	store, err := Open(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}

	defer store.Close()

	id := uuid.New()
	item := InventoryItem{Slot: 36, Item: "minecraft:diamond_sword", Count: 1, Components: []ItemComponent{{Type: 13, Data: []byte{1}}}, Removed: []int32{2}}

	if err = store.SavePlayer(id, PlayerData{Inventory: []InventoryItem{item}}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(store.playerPath(id))

	if err != nil {
		t.Fatal(err)
	}

	// The protocol number follows the header, position, rotation and held slot.
	w := protocol.NewWriter()
	w.VarInt(version.Protocol - 1)
	copy(data[4+1+3*8+2*4+1:], w.Bytes())

	if err = writeFileAtomic(store.playerPath(id), data); err != nil {
		t.Fatal(err)
	}

	got, _, err := store.LoadPlayer(id)
	want := []InventoryItem{{Slot: 36, Item: "minecraft:diamond_sword", Count: 1}}

	if err != nil || !reflect.DeepEqual(got.Inventory, want) {
		t.Fatalf("inventory loaded as %+v, %v, want %+v", got.Inventory, err, want)
	}
}

func TestRegionCompaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.0.0.region")
	r, err := openRegionFile(path)

	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewPCG(1, 2))
	record := func() []byte {
		b := make([]byte, 4096)

		for i := range b {
			b[i] = byte(rng.Uint32())
		}

		return b
	}

	keep := record()

	if err = r.write(7, keep); err != nil {
		t.Fatal(err)
	}

	// Rewriting one chunk leaves superseded records behind until compaction
	// reclaims them.
	var latest []byte
	var largest int64

	for range 600 {
		latest = record()

		if err = r.write(3, latest); err != nil {
			t.Fatal(err)
		}

		info, _ := os.Stat(path)
		largest = max(largest, info.Size())
	}

	info, _ := os.Stat(path)

	if largest > 2*minCompactWaste+regionHeaderSize || info.Size() >= largest {
		t.Fatalf("region file grew to %d bytes and is %d bytes now", largest, info.Size())
	}

	r.close()

	if r, err = openRegionFile(path); err != nil {
		t.Fatal(err)
	}

	defer r.close()

	for index, want := range map[int][]byte{3: latest, 7: keep} {
		if got, err := r.read(index); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("record %d after compaction: %v", index, err)
		}
	}
}
