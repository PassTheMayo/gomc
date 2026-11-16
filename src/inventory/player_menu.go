package inventory

import "main/src/item"

// Inventory menu slot ranges.
const (
	menuResult     = 0
	menuCraftStart = 1
	menuArmorStart = 5
	menuMainStart  = 9
	menuHotbar     = 36
	menuOffHand    = 45
	craftSize      = 4
)

// bindingCurse keeps armor on unless the player has infinite materials.
var bindingCurse = item.Enchantment("minecraft:binding_curse")

// storageSlot is a slot backed by a container slot.
type storageSlot struct {
	c *Container
	i int
}

func (s storageSlot) get() item.Stack          { return s.c.Get(s.i) }
func (s storageSlot) set(stack item.Stack)     { s.c.Set(s.i, stack) }
func (s storageSlot) mayPlace(item.Stack) bool { return true }
func (s storageSlot) mayPickup(bool) bool      { return true }
func (s storageSlot) maxStackSize() int32      { return maxSlotStack }

// armorSlot holds one piece of armor that is worn in its slot.
type armorSlot struct {
	storageSlot
	wears item.EquipmentSlot
}

func (s armorSlot) mayPlace(stack item.Stack) bool {
	return item.PropertiesOf(stack.Item).Equippable == s.wears
}

func (s armorSlot) mayPickup(creative bool) bool {
	return creative || s.get().EnchantmentLevel(bindingCurse) == 0
}

func (s armorSlot) maxStackSize() int32 { return 1 }

// resultSlot is the crafting result, which stays empty since the server has
// no recipes.
type resultSlot struct{}

func (resultSlot) get() item.Stack          { return item.Stack{} }
func (resultSlot) set(item.Stack)           {}
func (resultSlot) mayPlace(item.Stack) bool { return false }
func (resultSlot) mayPickup(bool) bool      { return true }
func (resultSlot) maxStackSize() int32      { return maxSlotStack }

// newMenu builds the inventory menu over the player's slots and crafting
// grid, in vanilla's InventoryMenu layout.
func (p *Player) newMenu() *menu {
	m := &menu{
		slots:  make([]slot, MenuSize),
		output: func(i int) bool { return i == menuResult },
		give:   func(s item.Stack) item.Stack { rest, _ := p.Add(s, false); return rest },
		hotbar: func(button int) slot { return storageSlot{p.slots, button} },
	}

	m.slots[menuResult] = resultSlot{}

	for i := range craftSize {
		m.slots[menuCraftStart+i] = storageSlot{p.craft, i}
	}

	for slot := range playerSize {
		s := storageSlot{p.slots, slot}

		if slot >= armorStart && slot < offHandSlot {
			m.slots[MenuSlot(slot)] = armorSlot{s, item.SlotFeet + item.EquipmentSlot(slot-armorStart)}
		} else {
			m.slots[MenuSlot(slot)] = s
		}
	}

	m.quickMove = func(i int) item.Stack { return p.quickMove(m, i) }

	return m
}

// quickMove moves a shift-clicked stack like vanilla's InventoryMenu: armor
// and off-hand items to where they are worn if free, the main inventory to
// the hotbar and back, and the crafting grid and armor into the inventory.
func (p *Player) quickMove(m *menu, i int) item.Stack {
	s := m.slots[i]
	original := s.get()

	if original.IsEmpty() {
		return item.Stack{}
	}

	worn := item.PropertiesOf(original.Item).Equippable
	armor := menuArmorStart + int(item.SlotHead-worn)

	var rest item.Stack
	var moved bool

	switch {
	case i < menuMainStart:
		rest, moved = m.moveTo(original, menuMainStart, menuOffHand, i == menuResult)
	case worn >= item.SlotFeet && worn <= item.SlotHead && m.slots[armor].get().IsEmpty():
		rest, moved = m.moveTo(original, armor, armor+1, false)
	case worn == item.SlotOffHand && m.slots[menuOffHand].get().IsEmpty():
		rest, moved = m.moveTo(original, menuOffHand, menuOffHand+1, false)
	case i < menuHotbar:
		rest, moved = m.moveTo(original, menuHotbar, menuOffHand, false)
	case i < menuOffHand:
		rest, moved = m.moveTo(original, menuMainStart, menuHotbar, false)
	default:
		rest, moved = m.moveTo(original, menuMainStart, menuOffHand, false)
	}

	if !moved || rest.Count == original.Count {
		return item.Stack{}
	}

	s.set(rest)

	return original
}

// Click applies a click in the inventory window like vanilla and returns the
// stacks the player threw out of it.
func (p *Player) Click(slot, button int, mode int32, creative bool) []item.Stack {
	return p.menu.click(slot, button, mode, creative)
}

// Carried returns the stack held on the cursor in the inventory window.
func (p *Player) Carried() item.Stack {
	return p.menu.carried
}

// Revision returns the inventory window's revision, which clients echo in
// clicks so the server can tell when they acted on outdated slots.
func (p *Player) Revision() int32 {
	return p.menu.revision
}

// NextRevision numbers a change sent to the client.
func (p *Player) NextRevision() int32 {
	return p.menu.nextRevision()
}

// CloseMenu puts the cursor stack and the crafting grid back into the
// inventory, like vanilla when the window closes. It returns the menu slots
// that changed and what didn't fit, which the player drops.
func (p *Player) CloseMenu() (changed []int, dropped []item.Stack) {
	back := []item.Stack{p.menu.carried}
	p.menu.carried = item.Stack{}
	p.menu.drag = drag{}

	for i := range craftSize {
		if s := p.craft.Get(i); !s.IsEmpty() {
			back = append(back, s)
			p.craft.Set(i, item.Stack{})
			changed = append(changed, menuCraftStart+i)
		}
	}

	for _, s := range back {
		if s.IsEmpty() {
			continue
		}

		rest, slots := p.Add(s, false)
		changed = append(changed, slots...)

		if !rest.IsEmpty() {
			dropped = append(dropped, rest)
		}
	}

	return changed, dropped
}
