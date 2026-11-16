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
