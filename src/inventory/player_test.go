package inventory

import (
	"reflect"
	"slices"
	"testing"

	"main/src/item"
	"main/src/registry"
)

func TestMenuSlots(t *testing.T) {
	seen := make(map[int]bool)

	for slot := range playerSize {
		menu := MenuSlot(slot)

		// The crafting result and grid take menu slots 0 to 4.
		if menu < menuArmorStart || menu >= MenuSize || seen[menu] {
			t.Errorf("slot %d shows at menu slot %d", slot, menu)
		}

		seen[menu] = true
	}

	// Head armor is at the top of the armor column.
	if MenuSlot(armorStart+3) != 5 || MenuSlot(armorStart) != 8 {
		t.Error("armor is not shown from head to feet")
	}
}

func TestPick(t *testing.T) {
	stone, dirt := item.Of(1, 64), item.Of(10, 1)
	enchanted := item.Stack{Count: 1, Item: 20, Components: []item.Component{
		{Type: registry.MustGet(registry.DataComponentType).MustID("minecraft:enchantments"), Data: []byte{1, 0, 1}},
	}}

	p := NewPlayer()
	p.slots.Set(0, stone)
	p.slots.Set(20, dirt)
	p.Select(4)

	// A matching hotbar stack is selected.
	if changed := p.Pick(item.Of(1, 1), true); changed != nil || p.Selected() != 0 {
		t.Fatalf("picking stone changed %v and selected %d", changed, p.Selected())
	}

	// A main inventory stack is swapped into the first empty hotbar slot.
	if changed := p.Pick(item.Of(10, 1), false); !slices.Equal(changed, []int{37, 20}) || !reflect.DeepEqual(p.MainHand(), dirt) {
		t.Fatalf("picking dirt changed %v and holds %+v", changed, p.MainHand())
	}

	// Without infinite materials nothing new is given.
	if changed := p.Pick(item.Of(30, 1), false); changed != nil {
		t.Fatalf("picking an item the player lacks changed %v", changed)
	}

	// With a full hotbar, the replaced stack skips enchanted ones and moves
	// to a free slot.
	for slot := range hotbarSize {
		p.slots.Set(slot, enchanted)
	}

	p.slots.Set(5, stone)
	p.Select(0)

	if changed := p.Pick(item.Of(30, 1), true); !slices.Equal(changed, []int{41, 9}) || !reflect.DeepEqual(p.slots.Get(9), stone) {
		t.Fatalf("picking into a full hotbar changed %v", changed)
	}
}

func TestSaveAndLoad(t *testing.T) {
	p := NewPlayer()
	p.Select(3)
	p.slots.Set(0, item.Stack{Count: 2, Item: 1, Components: []item.Component{{Type: 5, Data: []byte{9}}}, Removed: []int32{2}})
	p.slots.Set(offHandSlot, item.Of(10, 1))
	p.slots.Set(armorStart+3, item.Of(11, 1))

	loaded := NewPlayer()
	loaded.Load(p.Selected(), p.Save())

	if !reflect.DeepEqual(loaded.slots, p.slots) || loaded.Selected() != p.Selected() {
		t.Fatalf("loaded %+v, want %+v", loaded.slots, p.slots)
	}
}

func TestSwapHands(t *testing.T) {
	p := NewPlayer()
	p.Select(2)
	p.slots.Set(2, item.Of(1, 64))

	if changed := p.SwapHands(); !slices.Equal(changed, []int{38, 45}) {
		t.Fatalf("swapping changed %v", changed)
	}

	if !p.MainHand().IsEmpty() || !reflect.DeepEqual(p.OffHand(), item.Of(1, 64)) {
		t.Fatalf("after swapping the hands hold %+v and %+v", p.MainHand(), p.OffHand())
	}
}
