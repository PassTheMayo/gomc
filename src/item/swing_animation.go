package item

import (
	"slices"

	"main/src/protocol"
	"main/src/registry"
)

// Swing animation types.
const (
	SwingNone int32 = iota
	SwingWhack
	SwingStab
)

// SwingAnimation is how an arm swing looks: its type and length in ticks.
type SwingAnimation struct {
	Type     int32
	Duration int32
}

// DefaultSwing is the swing of items without their own animation.
var DefaultSwing = SwingAnimation{Type: SwingWhack, Duration: 6}

var (
	components         = registry.MustGet(registry.DataComponentType)
	attackAnimation    = components.MustID("minecraft:attack_animation")
	interactAnimation  = components.MustID("minecraft:interact_animation")
	defaultAttackSwing = make(map[int32]SwingAnimation)
)

// Spears stab. Their durations are the seconds passed to
// Item.Properties.spear times 20, rounded down in float32 like the jar.
var spearStabTicks = map[string]int32{
	"minecraft:wooden_spear":    13,
	"minecraft:stone_spear":     15,
	"minecraft:copper_spear":    17,
	"minecraft:iron_spear":      19,
	"minecraft:golden_spear":    19,
	"minecraft:diamond_spear":   21,
	"minecraft:netherite_spear": 23,
}

func init() {
	items := registry.MustGet(registry.Item)

	for name, ticks := range spearStabTicks {
		defaultAttackSwing[items.MustID(name)] = SwingAnimation{Type: SwingStab, Duration: ticks}
	}
}

// AttackAnimation returns the swing shown when the stack's holder punches.
func (s Stack) AttackAnimation() SwingAnimation {
	if swing, ok := defaultAttackSwing[s.Item]; ok {
		return s.swingComponent(attackAnimation, swing)
	}

	return s.swingComponent(attackAnimation, DefaultSwing)
}

// InteractAnimation returns the swing shown when the stack is used on a
// block. No item changes it by default.
func (s Stack) InteractAnimation() SwingAnimation {
	return s.swingComponent(interactAnimation, DefaultSwing)
}

// swingComponent reads a swing animation component: the stack's own value,
// the plain default if the stack removes it, or the item's default.
func (s Stack) swingComponent(typ int32, itemDefault SwingAnimation) SwingAnimation {
	if slices.Contains(s.Removed, typ) {
		return DefaultSwing
	}

	for _, c := range s.Components {
		if c.Type != typ {
			continue
		}

		r := protocol.NewReader(c.Data)
		swing := SwingAnimation{Type: r.VarInt(), Duration: r.VarInt()}

		if r.Err() != nil {
			return itemDefault
		}

		return swing
	}

	return itemDefault
}
