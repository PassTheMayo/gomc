package loot

import (
	"math/rand/v2"
	"slices"
	"testing"

	"main/src/block"
	"main/src/item"
	"main/src/protocol"
	"main/src/registry"
)

var itemRegistry = registry.MustGet(registry.Item)

// tool returns one of an item with the given enchantment levels.
func tool(name string, enchantments map[string]int32) item.Stack {
	s := item.Of(itemRegistry.MustID(name), 1)

	if len(enchantments) == 0 {
		return s
	}

	w := protocol.NewWriter()
	w.VarInt(int32(len(enchantments)))

	for e, level := range enchantments {
		w.VarInt(item.Enchantment(e))
		w.VarInt(level)
	}

	s.Components = []item.Component{{Type: registry.MustGet(registry.DataComponentType).MustID("minecraft:enchantments"), Data: w.Bytes()}}

	return s
}

// drops breaks a block and returns what dropped as item name and count pairs.
func drops(t *testing.T, ctx Context) map[string]int32 {
	t.Helper()

	if ctx.Rand == nil {
		ctx.Rand = rand.New(rand.NewPCG(1, 2))
	}

	out := make(map[string]int32)

	for _, s := range BlockDrops(&ctx) {
		out[itemRegistry.Entries[s.Item]] += s.Count
	}

	return out
}

func TestEveryTableDecodes(t *testing.T) {
	tables := 0

	for _, table := range blockTables() {
		if table != nil {
			tables++
		}
	}

	if tables != 1264 {
		t.Fatalf("%d blocks have loot tables, want 1264", tables)
	}
}

// Tags and loot tables name blocks by registry ID, which must be the block ID.
func TestBlockRegistryMatchesBlocks(t *testing.T) {
	if !slices.Equal(registry.MustGet(registry.Block).Entries, block.Names()) {
		t.Fatal("the block registry and the block palette list blocks in a different order")
	}
}

func TestBlockDrops(t *testing.T) {
	silkTouch := tool("minecraft:diamond_pickaxe", map[string]int32{"minecraft:silk_touch": 1})
	shears := tool("minecraft:shears", nil)
	state := block.MustParse

	cases := []struct {
		name string
		ctx  Context
		want map[string]int32
	}{
		{"stone", Context{State: state("stone"), ByEntity: true}, map[string]int32{"minecraft:cobblestone": 1}},
		{"stone with silk touch", Context{State: state("stone"), Tool: silkTouch}, map[string]int32{"minecraft:stone": 1}},
		{"glass", Context{State: state("glass")}, map[string]int32{}},
		{"glass with silk touch", Context{State: state("glass"), Tool: silkTouch}, map[string]int32{"minecraft:glass": 1}},
		{"sheared leaves", Context{State: state("oak_leaves"), Tool: shears}, map[string]int32{"minecraft:oak_leaves": 1}},
		{"seedling wheat", Context{State: state("wheat[age=0]")}, map[string]int32{"minecraft:wheat_seeds": 1}},
		{"upper door half", Context{State: state("oak_door[half=upper]")}, map[string]int32{}},
		{"lower door half", Context{State: state("oak_door[half=lower]")}, map[string]int32{"minecraft:oak_door": 1}},
		{"snow broken by a player", Context{State: state("snow[layers=3]"), ByEntity: true}, map[string]int32{"minecraft:snowball": 3}},
		{"snow without a breaker", Context{State: state("snow[layers=3]")}, map[string]int32{}},
		{"sea pickles", Context{State: state("sea_pickle[pickles=3]")}, map[string]int32{"minecraft:sea_pickle": 3}},
		{"wall torch", Context{State: state("wall_torch")}, map[string]int32{"minecraft:torch": 1}},
		{"air", Context{State: state("air")}, map[string]int32{}},
		{
			"sheared tall grass", Context{
				State: state("tall_grass[half=upper]"), Tool: shears,
				BlockAt: func(dx, dy, dz int) block.StateID { return state("tall_grass[half=lower]") },
			},
			map[string]int32{"minecraft:short_grass": 2},
		},
		{
			"tall grass missing its other half", Context{
				State: state("tall_grass[half=upper]"), Tool: shears,
				BlockAt: func(dx, dy, dz int) block.StateID { return state("air") },
			},
			map[string]int32{},
		},
	}

	for _, c := range cases {
		if got := drops(t, c.ctx); !mapsEqual(got, c.want) {
			t.Errorf("%s dropped %v, want %v", c.name, got, c.want)
		}
	}

	if got := drops(t, Context{State: state("wheat[age=7]")}); got["minecraft:wheat"] != 1 || got["minecraft:wheat_seeds"] < 1 {
		t.Errorf("grown wheat dropped %v", got)
	}
}

func mapsEqual(a, b map[string]int32) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}

// Fortune multiplies ore drops by 1 to level+1.
func TestFortune(t *testing.T) {
	pickaxe := tool("minecraft:diamond_pickaxe", map[string]int32{"minecraft:fortune": 3})
	r := rand.New(rand.NewPCG(3, 4))
	total := int32(0)

	for range 1000 {
		n := drops(t, Context{State: block.MustParse("iron_ore"), Tool: pickaxe, Rand: r})["minecraft:raw_iron"]

		if n < 1 || n > 4 {
			t.Fatalf("fortune III dropped %d raw iron", n)
		}

		total += n
	}

	// The expected count is 2.2.
	if total < 2000 || total > 2400 {
		t.Fatalf("fortune III dropped %d raw iron in 1000 ores", total)
	}
}

// Explosions destroy most drops, keeping each with a chance of 1/radius.
func TestExplosionDecay(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	survived := 0

	for range 1000 {
		if drops(t, Context{State: block.MustParse("stone"), ExplosionRadius: 4, Rand: r})["minecraft:cobblestone"] > 0 {
			survived++
		}
	}

	if survived < 200 || survived > 300 {
		t.Fatalf("%d of 1000 stone survived a radius 4 explosion", survived)
	}
}

// A beehive broken with silk touch keeps its honey level.
func TestCopyState(t *testing.T) {
	ctx := Context{
		State: block.MustParse("beehive[honey_level=5]"),
		Tool:  tool("minecraft:diamond_axe", map[string]int32{"minecraft:silk_touch": 1}),
		Rand:  rand.New(rand.NewPCG(1, 2)),
	}

	got := BlockDrops(&ctx)

	if len(got) != 1 || len(got[0].Components) != 1 || got[0].Components[0].Type != blockStateComponent {
		t.Fatalf("beehive dropped %+v", got)
	}

	r := protocol.NewReader(got[0].Components[0].Data)

	if n, key, value := r.VarInt(), r.String(64), r.String(64); n != 1 || key != "honey_level" || value != "5" {
		t.Fatalf("block state component holds %d entries, %s=%s", n, key, value)
	}
}
