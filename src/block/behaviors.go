package block

import "strings"

// behaviors lists every block family that differs from the defaults, in
// priority order. The rules follow the vanilla block classes.
var behaviors = []behavior{
	// Lightning and end rods and amethyst buds face the clicked face.
	{
		match:  nameMatches(func(n string) bool { return strings.HasSuffix(n, "_rod") || strings.Contains(n, "amethyst") }),
		facing: func(_ *Block, ctx PlaceContext, _ Direction) Direction { return ctx.Face },
	},

	// Trapdoors placed on a side face away from the block they are on.
	{
		match: nameMatches(func(n string) bool { return strings.HasSuffix(n, "_trapdoor") }),
		facing: func(_ *Block, ctx PlaceContext, player Direction) Direction {
			if ctx.Face.horizontal() {
				return ctx.Face
			}

			return player.Opposite()
		},
	},

	// Stairs, doors, fence gates and beds face the way the player looks.
	{
		match: nameMatches(func(n string) bool {
			return strings.HasSuffix(n, "_stairs") || strings.HasSuffix(n, "_door") ||
				strings.HasSuffix(n, "_fence_gate") || strings.HasSuffix(n, "_bed")
		}),
		facing: func(_ *Block, _ PlaceContext, player Direction) Direction { return player },
	},

	// Piston heads pick as the piston they belong to.
	{
		match: named("piston_head"),
		pickItem: func(b *Block, s StateID) string {
			if b.StateValues(s)["type"] == "sticky" {
				return "minecraft:sticky_piston"
			}

			return "minecraft:piston"
		},
	},

	// Flower pots with a plant pick as the plant.
	{
		match: nameMatches(func(n string) bool { return strings.HasPrefix(n, "potted_") }),
		pickItem: func(b *Block, _ StateID) string {
			plant := strings.TrimPrefix(b.Name, "minecraft:potted_")

			// potted_azalea_bush holds an azalea.
			if _, ok := byName["minecraft:"+plant]; !ok {
				plant = strings.TrimSuffix(plant, "_bush")
			}

			if p, ok := byName["minecraft:"+plant]; ok {
				return blockItems[p.ID]
			}

			return ""
		},
	},

	// The bodies of growing plants pick as their head.
	{
		match: named("kelp_plant", "weeping_vines_plant", "twisting_vines_plant"),
		pickItem: func(b *Block, _ StateID) string {
			return strings.TrimSuffix(b.Name, "_plant")
		},
	},

	pickedAs("cake", nameMatches(func(n string) bool { return strings.HasSuffix(n, "candle_cake") })),
	pickedAs("cauldron", named("water_cauldron", "lava_cauldron", "powder_snow_cauldron")),
	pickedAs("big_dripleaf", named("big_dripleaf_stem")),
	pickedAs("seagrass", named("tall_seagrass")),
	pickedAs("pumpkin_seeds", named("attached_pumpkin_stem")),
	pickedAs("melon_seeds", named("attached_melon_stem")),
	pickedAs("glow_berries", named("cave_vines_plant")),
	pickedAs("bamboo", named("bamboo_sapling")),
}

// pickedAs is a family of blocks that picks as one item.
func pickedAs(item string, match func(b *Block) bool) behavior {
	return behavior{match: match, pickItem: func(*Block, StateID) string { return "minecraft:" + item }}
}
