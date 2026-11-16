package inventory

import (
	"slices"
	"testing"

	"main/src/item"
	"main/src/packet"
)

func itemID(name string) int32 {
	return itemRegistry.MustID("minecraft:" + name)
}

func stack(name string, count int32) item.Stack {
	return item.Of(itemID(name), count)
}

// count returns the number of items in a menu slot.
func count(p *Player, menu int) int32 {
	return p.menu.slots[menu].get().Count
}

func TestPickupAndPlace(t *testing.T) {
	p := NewPlayer()
	p.SetMenuSlot(9, stack("stone", 10))

	// Right-clicking picks up half, rounded up; left-clicking the rest.
	p.Click(9, 1, packet.ClickPickup, false)

	if p.Carried().Count != 5 || count(p, 9) != 5 {
		t.Fatalf("right-click left %d and carried %d", count(p, 9), p.Carried().Count)
	}

	// Right-clicking an empty slot places one; left-clicking places the rest.
	p.Click(10, 1, packet.ClickPickup, false)
	p.Click(11, 0, packet.ClickPickup, false)

	if count(p, 10) != 1 || count(p, 11) != 4 || !p.Carried().IsEmpty() {
		t.Fatalf("placing left %d and %d, carrying %d", count(p, 10), count(p, 11), p.Carried().Count)
	}

	// Clicking a different item swaps it with the cursor.
	p.SetMenuSlot(12, stack("dirt", 3))
	p.Click(11, 0, packet.ClickPickup, false)
	p.Click(12, 0, packet.ClickPickup, false)

	if p.menu.slots[12].get().Item != itemID("stone") || p.Carried().Item != itemID("dirt") {
		t.Fatal("clicking another item with a full cursor must swap them")
	}
}

func TestClickOutsideThrows(t *testing.T) {
	p := NewPlayer()
	p.menu.carried = stack("stone", 5)

	if thrown := p.Click(int(packet.ClickOutside), 1, packet.ClickPickup, false); len(thrown) != 1 || thrown[0].Count != 1 || p.Carried().Count != 4 {
		t.Fatalf("right-clicking outside threw %+v", thrown)
	}

	if thrown := p.Click(int(packet.ClickOutside), 0, packet.ClickPickup, false); len(thrown) != 1 || thrown[0].Count != 4 || !p.Carried().IsEmpty() {
		t.Fatalf("left-clicking outside threw %+v", thrown)
	}
}

func TestQuickMove(t *testing.T) {
	p := NewPlayer()
	p.SetMenuSlot(9, stack("stone", 10))
	p.SetMenuSlot(37, stack("stone", 60))

	// The main inventory moves to the hotbar, topping up matching stacks.
	p.Click(9, 0, packet.ClickQuickMove, false)

	if count(p, 37) != 64 || count(p, 36) != 6 || count(p, 9) != 0 {
		t.Fatalf("shift-click left %d, %d and %d", count(p, 9), count(p, 36), count(p, 37))
	}

	// The hotbar moves to the main inventory.
	p.Click(36, 0, packet.ClickQuickMove, false)

	if count(p, 9) != 6 || count(p, 36) != 0 {
		t.Fatal("shift-clicking the hotbar must move to the main inventory")
	}

	// Armor goes to its slot.
	p.SetMenuSlot(20, stack("iron_helmet", 1))
	p.Click(20, 0, packet.ClickQuickMove, false)

	if p.menu.slots[5].get().Item != itemID("iron_helmet") {
		t.Fatal("shift-clicking a helmet must wear it")
	}
}

func TestArmorSlots(t *testing.T) {
	p := NewPlayer()
	p.menu.carried = stack("stone", 1)
	p.Click(5, 0, packet.ClickPickup, false)

	if count(p, 5) != 0 {
		t.Fatal("only helmets fit in the head slot")
	}

	p.menu.carried = stack("diamond_boots", 1)
	p.Click(8, 0, packet.ClickPickup, false)

	if count(p, 8) != 1 {
		t.Fatal("boots fit in the feet slot")
	}
}

func TestSwapWithHotbar(t *testing.T) {
	p := NewPlayer()
	p.SetMenuSlot(9, stack("stone", 10))
	p.SetMenuSlot(38, stack("dirt", 2))

	// Pressing 3 over slot 9 swaps it with the third hotbar slot.
	p.Click(9, 2, packet.ClickSwap, false)

	if p.slots.Get(2).Item != itemID("stone") || p.menu.slots[9].get().Item != itemID("dirt") {
		t.Fatal("the number keys must swap with the hotbar")
	}
}

func TestThrow(t *testing.T) {
	p := NewPlayer()
	p.SetMenuSlot(9, stack("stone", 10))

	if thrown := p.Click(9, 0, packet.ClickThrow, false); len(thrown) != 1 || thrown[0].Count != 1 || count(p, 9) != 9 {
		t.Fatalf("Q threw %+v", thrown)
	}

	if thrown := p.Click(9, 1, packet.ClickThrow, false); len(thrown) != 1 || thrown[0].Count != 9 || count(p, 9) != 0 {
		t.Fatalf("Ctrl-Q threw %+v", thrown)
	}
}

func TestDrag(t *testing.T) {
	p := NewPlayer()
	p.menu.carried = stack("stone", 10)

	// A left drag over three slots splits the stack evenly.
	for _, step := range []struct{ slot, button int }{{-999, 0}, {9, 1}, {10, 1}, {11, 1}, {-999, 2}} {
		p.Click(step.slot, step.button, packet.ClickQuickCraft, false)
	}

	if count(p, 9) != 3 || count(p, 10) != 3 || count(p, 11) != 3 || p.Carried().Count != 1 {
		t.Fatalf("dragging left %d, %d, %d and carried %d", count(p, 9), count(p, 10), count(p, 11), p.Carried().Count)
	}

	// Cloning stacks by dragging needs infinite materials.
	for _, step := range []struct{ slot, button int }{{-999, 8}, {12, 9}, {13, 9}, {-999, 10}} {
		p.Click(step.slot, step.button, packet.ClickQuickCraft, false)
	}

	if count(p, 12) != 0 {
		t.Fatal("survival players can't clone stacks")
	}
}

func TestGather(t *testing.T) {
	p := NewPlayer()
	p.SetMenuSlot(9, stack("stone", 64))
	p.SetMenuSlot(10, stack("stone", 5))
	p.menu.carried = stack("stone", 1)

	// Double-clicking takes partial stacks before full ones.
	p.Click(11, 0, packet.ClickPickupAll, false)

	if p.Carried().Count != 64 || count(p, 10) != 0 || count(p, 9) != 6 {
		t.Fatalf("gathering carried %d, leaving %d and %d", p.Carried().Count, count(p, 9), count(p, 10))
	}
}

func TestAdd(t *testing.T) {
	p := NewPlayer()
	p.Select(4)
	p.slots.Set(4, stack("stone", 60))
	p.slots.Set(offHandSlot, stack("stone", 60))
	p.slots.Set(0, stack("stone", 60))

	// The selected slot fills first, then the off hand, then other slots.
	rest, changed := p.Add(stack("stone", 10), false)

	if !rest.IsEmpty() || !slices.Equal(changed, []int{MenuSlot(4), MenuSlot(offHandSlot), MenuSlot(0)}) || p.slots.Get(0).Count != 62 {
		t.Fatalf("adding changed %v", changed)
	}

	// What doesn't fit is left over.
	full := NewPlayer()

	for slot := range itemSlots {
		full.slots.Set(slot, stack("dirt", 64))
	}

	if rest, _ := full.Add(stack("stone", 5), false); rest.Count != 5 {
		t.Fatalf("a full inventory kept %d", rest.Count)
	}

	if rest, _ := full.Add(stack("stone", 5), true); !rest.IsEmpty() {
		t.Fatal("infinite materials take what doesn't fit")
	}
}

func TestCloseMenu(t *testing.T) {
	p := NewPlayer()
	p.menu.carried = stack("stone", 3)
	p.SetMenuSlot(1, stack("dirt", 2))
	changed, dropped := p.CloseMenu()

	if len(dropped) != 0 || !p.Carried().IsEmpty() || count(p, 1) != 0 || p.slots.Get(0).Count != 3 || p.slots.Get(1).Count != 2 {
		t.Fatalf("closing the menu changed %v and dropped %+v", changed, dropped)
	}
}
