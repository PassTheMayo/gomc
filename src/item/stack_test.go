package item

import (
	"testing"

	"main/src/registry"
)

func TestSameItemSameComponents(t *testing.T) {
	sword := Stack{Count: 1, Item: 5, Components: []Component{{Type: 1, Data: []byte{1}}, {Type: 2, Data: []byte{2}}}}
	reordered := Stack{Count: 3, Item: 5, Components: []Component{{Type: 2, Data: []byte{2}}, {Type: 1, Data: []byte{1}}}}

	cases := []struct {
		a, b Stack
		want bool
	}{
		{Of(5, 1), Of(5, 64), true},
		{Of(5, 1), Of(6, 1), false},
		{Of(5, 1), Stack{}, false},
		{sword, reordered, true},
		{sword, Of(5, 1), false},
		{Stack{Count: 1, Item: 5, Removed: []int32{3}}, Of(5, 1), false},
	}

	for _, c := range cases {
		if got := SameItemSameComponents(c.a, c.b); got != c.want {
			t.Errorf("SameItemSameComponents(%+v, %+v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestIsEnchanted(t *testing.T) {
	if !(Stack{Count: 1, Components: []Component{{Type: enchantments, Data: []byte{1, 0, 1}}}}).IsEnchanted() {
		t.Error("a stack with an enchantment is not enchanted")
	}

	if (Stack{Count: 1, Components: []Component{{Type: enchantments, Data: []byte{0}}}}).IsEnchanted() {
		t.Error("a stack with an empty enchantments component is enchanted")
	}
}

func TestSwingAnimations(t *testing.T) {
	items := registry.MustGet(registry.Item)
	spear := Of(items.MustID("minecraft:iron_spear"), 1)
	custom := Stack{Count: 1, Item: 1, Components: []Component{{Type: attackAnimation, Data: []byte{byte(SwingStab), 30}}}}

	cases := []struct {
		got, want SwingAnimation
	}{
		{Of(1, 1).AttackAnimation(), DefaultSwing},
		{spear.AttackAnimation(), SwingAnimation{Type: SwingStab, Duration: 19}},
		{spear.InteractAnimation(), DefaultSwing},
		{custom.AttackAnimation(), SwingAnimation{Type: SwingStab, Duration: 30}},
		{Stack{Count: 1, Item: spear.Item, Removed: []int32{attackAnimation}}.AttackAnimation(), DefaultSwing},
	}

	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %+v, want %+v", i, c.got, c.want)
		}
	}
}

func TestProperties(t *testing.T) {
	items := registry.MustGet(registry.Item)
	blocks := registry.MustGet(registry.Block)
	id := func(name string) int32 { return items.MustID(name) }

	if got := Of(id("minecraft:ender_pearl"), 1).MaxStackSize(); got != 16 {
		t.Errorf("ender pearls stack to %d", got)
	}

	if got := Of(id("minecraft:stone"), 1).MaxStackSize(); got != 64 {
		t.Errorf("stone stacks to %d", got)
	}

	if PropertiesOf(id("minecraft:iron_helmet")).Equippable != SlotHead || PropertiesOf(id("minecraft:shield")).Equippable != SlotOffHand {
		t.Error("armor and shields must know the slot they are worn in")
	}

	// A max_stack_size component overrides the default.
	custom := Stack{Count: 1, Item: id("minecraft:stone"), Components: []Component{{Type: maxStackSize, Data: []byte{99}}}}

	if custom.MaxStackSize() != 99 {
		t.Errorf("a stack size component of 99 gave %d", custom.MaxStackSize())
	}

	stone, obsidian, cobweb := blocks.MustID("minecraft:stone"), blocks.MustID("minecraft:obsidian"), blocks.MustID("minecraft:cobweb")

	cases := []struct {
		tool  string
		block int32
		want  bool
	}{
		{"minecraft:wooden_pickaxe", stone, true},
		{"minecraft:wooden_pickaxe", obsidian, false},
		{"minecraft:diamond_pickaxe", obsidian, true},
		{"minecraft:diamond_shovel", stone, false},
		{"minecraft:shears", cobweb, true},
		{"minecraft:stick", stone, false},
	}

	for _, c := range cases {
		if got := PropertiesOf(id(c.tool)).CorrectForDrops(c.block); got != c.want {
			t.Errorf("%s harvesting block %d = %v, want %v", c.tool, c.block, got, c.want)
		}
	}
}

func TestEnchantmentLevel(t *testing.T) {
	enchantments := registry.MustGet(registry.DataComponentType).MustID("minecraft:enchantments")
	fortune, silk := Enchantment("minecraft:fortune"), Enchantment("minecraft:silk_touch")
	s := Stack{Count: 1, Item: 1, Components: []Component{{Type: enchantments, Data: []byte{1, byte(fortune), 3}}}}

	if s.EnchantmentLevel(fortune) != 3 || s.EnchantmentLevel(silk) != 0 || Of(1, 1).EnchantmentLevel(fortune) != 0 {
		t.Fatalf("enchantment levels fortune=%d silk_touch=%d", s.EnchantmentLevel(fortune), s.EnchantmentLevel(silk))
	}
}
