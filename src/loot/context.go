// Package loot generates items from vanilla loot tables. Conditions,
// modifiers and number providers are looked up by their JSON type, so new
// types plug in by registering a decoder.
package loot

import (
	"math/rand/v2"

	"main/src/block"
	"main/src/item"
)

// Context is what a loot table can inspect while generating items.
type Context struct {
	// State is the block whose loot is generated.
	State block.StateID

	// Tool is the stack the block was broken with, empty if none.
	Tool item.Stack

	// ByEntity reports whether an entity, such as a player, broke the block.
	ByEntity bool

	// ExplosionRadius is the radius of the explosion that broke the block, or
	// 0 if it wasn't an explosion.
	ExplosionRadius float32

	// BlockAt returns the block at an offset from the broken block.
	BlockAt func(dx, dy, dz int) block.StateID

	// Rand is the source of every random roll.
	Rand *rand.Rand
}

// enchantmentLevel returns the level of an enchantment on the tool.
func (c *Context) enchantmentLevel(enchantment int32) int32 {
	if c.Tool.IsEmpty() {
		return 0
	}

	return c.Tool.EnchantmentLevel(enchantment)
}
