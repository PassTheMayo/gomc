package block

import "testing"

// Expected values were computed from the 26.3 server jar's block registration
// order and cross-checked against the published 26.1 state table.
func TestKnownStateIDs(t *testing.T) {
	cases := map[string]StateID{
		"minecraft:air":                      0,
		"minecraft:stone":                    1,
		"minecraft:grass_block[snowy=false]": 9,
		"minecraft:dirt":                     10,
	}

	for spec, want := range cases {
		got, err := Parse(spec)

		if err != nil {
			t.Fatalf("Parse(%q): %v", spec, err)
		}

		if got != want {
			t.Errorf("Parse(%q) = %d, want %d", spec, got, want)
		}
	}

	if StateCount() != 35723 {
		t.Errorf("StateCount() = %d, want 35723", StateCount())
	}
}

func TestStateRoundTrip(t *testing.T) {
	for s := StateID(0); int(s) < StateCount(); s += 97 {
		parsed, err := Parse(s.String())

		if err != nil {
			t.Fatalf("Parse(%q): %v", s.String(), err)
		}

		if parsed != s {
			t.Fatalf("Parse(%q) = %d, want %d", s.String(), parsed, s)
		}
	}
}

func TestBehavior(t *testing.T) {
	water := MustParse("water")
	leaves := MustParse("oak_leaves[waterlogged=true]")

	if !HasFluid(water) || !BlocksMotion(water) || LightOpacity(water) != 1 {
		t.Error("water should be a motion blocking fluid with opacity 1")
	}

	if !HasFluid(leaves) {
		t.Error("waterlogged leaves should contain a fluid")
	}

	if !IsAir(MustParse("cave_air")) || BlocksMotion(MustParse("short_grass")) {
		t.Error("air and plants should not block motion")
	}

	if LightOpacity(MustParse("stone")) != 15 {
		t.Error("stone should be opaque")
	}
}

func TestPickItem(t *testing.T) {
	cases := map[string]string{
		"stone":                              "minecraft:stone",
		"wheat[age=7]":                       "minecraft:wheat_seeds",
		"redstone_wire":                      "minecraft:redstone",
		"wall_torch[facing=north]":           "minecraft:torch",
		"oak_wall_hanging_sign":              "minecraft:oak_hanging_sign",
		"white_wall_banner":                  "minecraft:white_banner",
		"tube_coral_wall_fan":                "minecraft:tube_coral_fan",
		"skeleton_wall_skull":                "minecraft:skeleton_skull",
		"water_cauldron[level=2]":            "minecraft:cauldron",
		"potted_azalea_bush":                 "minecraft:azalea",
		"potted_bamboo":                      "minecraft:bamboo",
		"flower_pot":                         "minecraft:flower_pot",
		"white_candle_cake":                  "minecraft:cake",
		"piston_head[type=sticky,facing=up]": "minecraft:sticky_piston",
		"piston_head[type=normal,facing=up]": "minecraft:piston",
		"attached_melon_stem[facing=east]":   "minecraft:melon_seeds",
		"cave_vines_plant[berries=true]":     "minecraft:glow_berries",
		"oak_door[half=upper,facing=north]":  "minecraft:oak_door",
	}

	for spec, want := range cases {
		if got, ok := PickItem(MustParse(spec)); !ok || got != want {
			t.Errorf("PickItem(%s) = %q, %v, want %q", spec, got, ok, want)
		}
	}

	// Exactly the blocks vanilla gives no item for.
	none := map[string]bool{
		"minecraft:air": true, "minecraft:void_air": true, "minecraft:cave_air": true,
		"minecraft:water": true, "minecraft:lava": true, "minecraft:bubble_column": true,
		"minecraft:fire": true, "minecraft:soul_fire": true, "minecraft:moving_piston": true,
		"minecraft:nether_portal": true, "minecraft:end_portal": true, "minecraft:end_gateway": true,
		"minecraft:frosted_ice": true,
	}

	for _, name := range Names() {
		b, _ := ByName(name)

		if _, ok := PickItem(b.DefaultState); ok == none[name] {
			t.Errorf("PickItem(%s) found an item: %v", name, ok)
		}
	}
}

// Each hook comes from the first matching behavior that defines it, and
// blocks without one use the default.
func TestBehaviorResolution(t *testing.T) {
	saved := behaviors
	defer func() {
		behaviors = saved
		resolveHooks()
	}()

	behaviors = []behavior{
		{match: named("stone"), pickItem: func(*Block, StateID) string { return "first" }},
		{match: named("stone", "dirt"), pickItem: func(*Block, StateID) string { return "second" }},
	}
	resolveHooks()

	cases := map[string]string{"stone": "first", "dirt": "second", "oak_planks": "minecraft:oak_planks"}

	for name, want := range cases {
		if got, _ := PickItem(MustParse(name)); got != want {
			t.Errorf("%s picks as %q, want %q", name, got, want)
		}
	}
}
