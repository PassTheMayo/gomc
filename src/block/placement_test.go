package block

import "testing"

func place(t *testing.T, item string, ctx PlaceContext) []Placement {
	t.Helper()

	b, ok := ForItem(item)

	if !ok {
		t.Fatalf("%s is not a block item", item)
	}

	return b.Place(ctx)
}

func TestPlacement(t *testing.T) {
	air := MustParse("air")
	cases := []struct {
		item string
		ctx  PlaceContext
		want []string
	}{
		{"minecraft:stone", PlaceContext{Face: Up, Replacing: air}, []string{"minecraft:stone"}},
		{"minecraft:oak_log", PlaceContext{Face: East}, []string{"minecraft:oak_log[axis=x]"}},
		{"minecraft:oak_log", PlaceContext{Face: Up}, []string{"minecraft:oak_log[axis=y]"}},
		// Facing south (yaw 0), a furnace faces back at the player.
		{"minecraft:furnace", PlaceContext{Face: Up, Yaw: 0}, []string{"minecraft:furnace[facing=north,lit=false]"}},
		{"minecraft:oak_stairs", PlaceContext{Face: North, Yaw: 90, CursorY: 0.8},
			[]string{"minecraft:oak_stairs[facing=west,half=top,shape=straight,waterlogged=false]"}},
		{"minecraft:stone_slab", PlaceContext{Face: Down}, []string{"minecraft:stone_slab[type=top,waterlogged=false]"}},
		{"minecraft:stone_slab", PlaceContext{Face: Up, Replacing: waterSource}, []string{"minecraft:stone_slab[type=bottom,waterlogged=true]"}},
		{"minecraft:torch", PlaceContext{Face: Up}, []string{"minecraft:torch"}},
		{"minecraft:torch", PlaceContext{Face: West}, []string{"minecraft:wall_torch[facing=west]"}},
		{"minecraft:light_blue_banner", PlaceContext{Face: South}, []string{"minecraft:light_blue_wall_banner[facing=south]"}},
		{"minecraft:oak_sign", PlaceContext{Face: Up, Yaw: 0}, []string{"minecraft:oak_sign[rotation=8,waterlogged=false]"}},
		{"minecraft:stone_button", PlaceContext{Face: North}, []string{"minecraft:stone_button[face=wall,facing=north,powered=false]"}},
		{"minecraft:oak_door", PlaceContext{Face: Up, Yaw: -90}, []string{
			"minecraft:oak_door[facing=east,half=lower,hinge=left,open=false,powered=false]",
			"minecraft:oak_door[facing=east,half=upper,hinge=left,open=false,powered=false]",
		}},
		{"minecraft:red_bed", PlaceContext{Face: Up, Yaw: 0}, []string{
			"minecraft:red_bed[facing=south,occupied=false,part=foot]",
			"minecraft:red_bed[facing=south,occupied=false,part=head]",
		}},
		{"minecraft:string", PlaceContext{Face: Up}, []string{"minecraft:tripwire[attached=false,disarmed=false,east=false,north=false,powered=false,south=false,west=false]"}},
	}

	for _, c := range cases {
		got := place(t, c.item, c.ctx)

		if len(got) != len(c.want) {
			t.Errorf("%s placed %d blocks, want %d", c.item, len(got), len(c.want))
			continue
		}

		for i, p := range got {
			if p.State.String() != c.want[i] {
				t.Errorf("%s part %d = %s, want %s", c.item, i, p.State, c.want[i])
			}
		}
	}

	bed := place(t, "minecraft:red_bed", PlaceContext{Face: Up, Yaw: 0})

	if bed[1].DZ != 1 || bed[1].DX != 0 {
		t.Errorf("a bed facing south has its head to the south, got offset %d,%d", bed[1].DX, bed[1].DZ)
	}
}

func TestLinkedPart(t *testing.T) {
	if _, dy, _, ok := LinkedPart(MustParse("oak_door[half=upper]")); !ok || dy != -1 {
		t.Error("the upper half of a door links to the block below")
	}

	if dx, _, _, ok := LinkedPart(MustParse("red_bed[part=head,facing=east]")); !ok || dx != -1 {
		t.Error("the head of an east facing bed links to the block to its west")
	}

	if _, _, _, ok := LinkedPart(MustParse("stone")); ok {
		t.Error("stone is a single block")
	}
}

func TestItemsAndEntities(t *testing.T) {
	if _, ok := ForItem("minecraft:diamond"); ok {
		t.Error("a diamond is not a block item")
	}

	if b, _ := ForItem("minecraft:redstone"); b == nil || b.Name != "minecraft:redstone_wire" {
		t.Error("redstone places redstone wire")
	}

	if typ, ok := EntityType(MustParse("chest")); !ok || typ != "minecraft:chest" {
		t.Errorf("chest block entity = %q, %v", typ, ok)
	}

	if _, ok := EntityType(MustParse("stone")); ok {
		t.Error("stone has no block entity")
	}

	if !IsReplaceable(MustParse("short_grass")) || IsReplaceable(MustParse("stone")) || !IsSolid(MustParse("stone")) {
		t.Error("unexpected replaceable or solid flags")
	}
}
