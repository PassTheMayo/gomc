package inventory

import (
	"main/src/item"
	"main/src/registry"
	"main/src/storage"
)

// Player inventory slots, numbered like vanilla's Inventory: the hotbar
// first, then the main inventory, armor from feet to head and the off hand.
const (
	hotbarSize  = 9
	itemSlots   = 36
	armorStart  = 36
	offHandSlot = 40
	playerSize  = 41

	// MenuSize is the number of slots in the player's inventory menu.
	MenuSize = 46
)

var itemRegistry = registry.MustGet(registry.Item)

// Player is a player's inventory, selected hotbar slot and inventory window
// with its crafting grid. It isn't safe for concurrent use.
type Player struct {
	slots    *Container
	craft    *Container
	selected int
	menu     *menu
}

// NewPlayer returns an empty player inventory with the first hotbar slot
// selected.
func NewPlayer() *Player {
	p := &Player{slots: NewContainer(playerSize), craft: NewContainer(craftSize)}
	p.menu = p.newMenu()

	return p
}

// MenuSlot converts an inventory slot to its inventory menu slot. The menu
// shows the crafting grid in slots 0 to 4, armor from head to feet in 5 to 8,
// the main inventory in 9 to 35, the hotbar in 36 to 44 and the off hand in 45.
func MenuSlot(slot int) int {
	switch {
	case slot < hotbarSize:
		return 36 + slot
	case slot < itemSlots:
		return slot
	case slot < offHandSlot:
		return 8 - (slot - armorStart)
	default:
		return 45
	}
}

// Selected returns the selected hotbar slot.
func (p *Player) Selected() int {
	return p.selected
}

// Select selects a hotbar slot, from 0 to 8, and reports whether it was valid.
func (p *Player) Select(slot int) bool {
	if slot < 0 || slot >= hotbarSize {
		return false
	}

	p.selected = slot

	return true
}

// MainHand returns the stack in the selected hotbar slot.
func (p *Player) MainHand() item.Stack {
	return p.slots.Get(p.selected)
}

// OffHand returns the stack in the off hand.
func (p *Player) OffHand() item.Stack {
	return p.slots.Get(offHandSlot)
}

// SetMenuSlot puts a stack in an inventory menu slot, from the crafting grid
// to the off hand, and reports whether the slot exists.
func (p *Player) SetMenuSlot(menu int, s item.Stack) bool {
	if menu < menuCraftStart || menu >= MenuSize {
		return false
	}

	p.menu.slots[menu].set(s)

	return true
}

// Menu returns the stacks of every inventory menu slot.
func (p *Player) Menu() []item.Stack {
	menu := make([]item.Stack, MenuSize)

	for i, s := range p.menu.slots {
		menu[i] = s.get()
	}

	return menu
}

// SwapHands exchanges the stacks in the main hand and the off hand and
// returns the menu slots that changed.
func (p *Player) SwapHands() []int {
	p.slots.Swap(p.selected, offHandSlot)

	return []int{MenuSlot(p.selected), MenuSlot(offHandSlot)}
}

// DropSelected removes the whole selected stack, or one of its items, and
// returns what it removed.
func (p *Player) DropSelected(all bool) item.Stack {
	s := p.slots.Get(p.selected)

	if s.IsEmpty() {
		return item.Stack{}
	}

	n := int32(1)

	if all {
		n = s.Count
	}

	p.slots.Set(p.selected, s.WithCount(s.Count-n))

	return s.WithCount(n)
}

// Add puts a stack into the inventory like vanilla's Inventory.add: onto a
// matching stack with room, trying the selected slot and the off hand first,
// else into the first free slot, until it all fits or nothing more does.
// Players with infinite materials lose what doesn't fit at all. It returns
// what is left and the menu slots that changed.
func (p *Player) Add(s item.Stack, infinite bool) (item.Stack, []int) {
	var changed []int
	original := s.Count

	for !s.IsEmpty() {
		slot := p.slotWithRoomFor(s)

		if slot < 0 {
			slot = p.slots.Find(0, itemSlots, isEmptySlot)
		}

		if slot < 0 {
			break
		}

		current := p.slots.Get(slot)

		if current.IsEmpty() {
			current = s.WithCount(0)
		}

		n := min(s.Count, min(maxSlotStack, current.MaxStackSize())-current.Count)

		if n <= 0 {
			break
		}

		p.slots.Set(slot, current.WithCount(current.Count+n))
		s.Count -= n
		changed = append(changed, MenuSlot(slot))
	}

	if infinite && s.Count == original {
		s.Count = 0
	}

	return s, changed
}

// slotWithRoomFor returns the slot holding a stack the given one can join,
// or -1.
func (p *Player) slotWithRoomFor(s item.Stack) int {
	hasRoom := func(o item.Stack) bool {
		return item.SameItemSameComponents(o, s) && o.IsStackable() && o.Count < min(maxSlotStack, o.MaxStackSize())
	}

	for _, slot := range []int{p.selected, offHandSlot} {
		if hasRoom(p.slots.Get(slot)) {
			return slot
		}
	}

	return p.slots.Find(0, itemSlots, hasRoom)
}

// Pick moves a stack into the player's hand like vanilla's tryPickItem: a
// matching stack in the hotbar is selected, one in the main inventory is
// swapped into the hotbar, and otherwise players with infinite materials get
// the stack. It returns the menu slots that changed.
func (p *Player) Pick(s item.Stack, infinite bool) []int {
	matching := func(o item.Stack) bool { return item.SameItemSameComponents(o, s) }

	switch slot := p.slots.Find(0, itemSlots, matching); {
	case slot >= 0 && slot < hotbarSize:
		p.selected = slot

		return nil
	case slot >= 0:
		p.selected = p.suitableHotbarSlot()
		p.slots.Swap(p.selected, slot)

		return []int{MenuSlot(p.selected), MenuSlot(slot)}
	case infinite:
		p.selected = p.suitableHotbarSlot()
		changed := []int{MenuSlot(p.selected)}

		// The replaced stack moves to a free slot if there is one.
		if free := p.slots.Find(0, itemSlots, isEmptySlot); free >= 0 && !p.slots.Get(p.selected).IsEmpty() {
			p.slots.Set(free, p.slots.Get(p.selected))
			changed = append(changed, MenuSlot(free))
		}

		p.slots.Set(p.selected, s)

		return changed
	}

	return nil
}

// suitableHotbarSlot returns the hotbar slot a picked stack goes to: the first
// empty one from the selected slot onwards, else the first without an
// enchanted stack, else the selected one.
func (p *Player) suitableHotbarSlot() int {
	for _, wanted := range []func(item.Stack) bool{isEmptySlot, func(s item.Stack) bool { return !s.IsEnchanted() }} {
		for i := range hotbarSize {
			if slot := (p.selected + i) % hotbarSize; wanted(p.slots.Get(slot)) {
				return slot
			}
		}
	}

	return p.selected
}

// Save returns the non-empty stacks in their saved form, by menu slot.
func (p *Player) Save() []storage.InventoryItem {
	var items []storage.InventoryItem

	for slot := range playerSize {
		s := p.slots.Get(slot)

		if s.IsEmpty() {
			continue
		}

		saved := storage.InventoryItem{Slot: MenuSlot(slot), Item: itemRegistry.Entries[s.Item], Count: s.Count, Removed: s.Removed}

		for _, c := range s.Components {
			saved.Components = append(saved.Components, storage.ItemComponent(c))
		}

		items = append(items, saved)
	}

	return items
}

// Load restores a saved inventory and selected slot, skipping items that no
// longer exist.
func (p *Player) Load(selected int, items []storage.InventoryItem) {
	p.Select(selected)

	for _, saved := range items {
		id, ok := itemRegistry.ID(saved.Item)

		if !ok || id == 0 || saved.Count <= 0 {
			continue
		}

		s := item.Stack{Count: saved.Count, Item: id, Removed: saved.Removed}

		for _, c := range saved.Components {
			s.Components = append(s.Components, item.Component(c))
		}

		p.SetMenuSlot(saved.Slot, s)
	}
}
