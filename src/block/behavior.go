package block

import "strings"

// behavior is how a family of blocks acts, such as stairs facing the player
// or crops picking as seeds. Every hook is optional: a block takes each hook
// from the first behavior in behaviors that matches it and defines that hook,
// or from defaultHooks. A new mechanic adds a hook here and to hooks, a
// default, and behaviors for the blocks it affects.
type behavior struct {
	// match selects the blocks the behavior applies to.
	match func(b *Block) bool

	// facing picks the facing of a newly placed block that has a facing
	// property, given the horizontal direction the player looks in.
	facing func(b *Block, ctx PlaceContext, player Direction) Direction

	// pickItem returns the item middle-clicking state s gives, or "" for none.
	pickItem func(b *Block, s StateID) string
}

// hooks are a block's resolved behavior.
type hooks struct {
	facing   func(b *Block, ctx PlaceContext, player Direction) Direction
	pickItem func(b *Block, s StateID) string
}

var defaultHooks = hooks{
	// Blocks facing any of six directions face away from where the player
	// looks; others, like furnaces and chests, face the player.
	facing: func(b *Block, ctx PlaceContext, player Direction) Direction {
		if values := propertyValues(b, "facing"); len(values) == 6 {
			return lookDirection(ctx.Yaw, ctx.Pitch).Opposite()
		}

		return player.Opposite()
	},

	// Blocks pick as the item that places them, which covers wall-mounted
	// variants too.
	pickItem: func(b *Block, _ StateID) string {
		return blockItems[b.ID]
	},
}

// resolveHooks gives every block its hooks. It runs once the block table and
// items are loaded.
func resolveHooks() {
	for _, b := range blocks {
		b.hooks = defaultHooks
		var facingSet, pickSet bool

		for _, beh := range behaviors {
			if !beh.match(b) {
				continue
			}

			if beh.facing != nil && !facingSet {
				b.hooks.facing, facingSet = beh.facing, true
			}

			if beh.pickItem != nil && !pickSet {
				b.hooks.pickItem, pickSet = beh.pickItem, true
			}
		}
	}
}

// named matches blocks by their names without the namespace.
func named(names ...string) func(b *Block) bool {
	set := make(map[string]bool, len(names))

	for _, n := range names {
		set["minecraft:"+n] = true
	}

	return func(b *Block) bool { return set[b.Name] }
}

// nameMatches matches blocks whose name without the namespace satisfies test,
// for families named by a pattern.
func nameMatches(test func(name string) bool) func(b *Block) bool {
	return func(b *Block) bool { return test(strings.TrimPrefix(b.Name, "minecraft:")) }
}

// propertyValues returns the values of a block's property, or nil.
func propertyValues(b *Block, name string) []string {
	for _, p := range b.Properties {
		if p.Name == name {
			return p.Values
		}
	}

	return nil
}

// hasProperty reports whether a block has a property.
func hasProperty(b *Block, name string) bool {
	return propertyValues(b, name) != nil
}
