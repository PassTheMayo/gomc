package registry

import "testing"

func TestBuiltinRegistries(t *testing.T) {
	cases := map[string]map[string]int32{
		// IDs derived from the 26.3 registration order.
		Block:      {"minecraft:air": 0, "minecraft:stone": 1},
		EntityType: {"minecraft:player": 159},
		Item:       {"minecraft:air": 0, "minecraft:stone": 1},

		DataComponentType: {"minecraft:custom_data": 0, "minecraft:enchantments": 13},
	}

	for name, ids := range cases {
		r := MustGet(name)

		for entry, want := range ids {
			if got := r.MustID(entry); got != want {
				t.Errorf("%s %s has ID %d, want %d", name, entry, got, want)
			}
		}
	}

	if n := len(MustGet(Item).Entries); n != 1658 {
		t.Errorf("item registry has %d entries, want 1658", n)
	}
}

// The client parses its own copy of the vanilla data for synchronized
// registries and fails if any tag that data references was not sent.
func TestTagsReferencedByVanillaData(t *testing.T) {
	required := map[string][]string{
		Item:                         {"minecraft:enchantable/head_armor", "minecraft:sulfur_cube_archetype/bouncy"},
		Block:                        {"minecraft:infiniburn_overworld", "minecraft:soul_speed_blocks"},
		"minecraft:timeline":         {"minecraft:in_overworld"},
		"minecraft:enchantment":      {"minecraft:exclusive_set/armor"},
		"minecraft:damage_type":      {"minecraft:bypasses_invulnerability"},
		"minecraft:worldgen/biome":   {"minecraft:spawns_warm_variant_farm_animals"},
		"minecraft:dialog":           {"minecraft:pause_screen_additions"},
		"minecraft:fluid":            {"minecraft:water"},
		"minecraft:entity_type":      {"minecraft:skeletons"},
		"minecraft:banner_pattern":   {"minecraft:no_item_required"},
		"minecraft:painting_variant": {"minecraft:placeable"},
	}

	tags := NetworkTags()

	for reg, names := range required {
		for _, name := range names {
			found := false

			for _, tag := range tags[reg] {
				found = found || tag.Name == name
			}

			if !found {
				t.Errorf("tag %s of %s is not sent", name, reg)
			}
		}
	}

	for _, r := range Synchronized() {
		if len(r.Entries) == 0 {
			t.Errorf("synchronized registry %s is empty", r.Name)
		}
	}
}
