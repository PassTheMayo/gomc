package server

import (
	"main/src/block"
	"main/src/packet"
	"main/src/registry"
	"main/src/storage"
)

const (
	// Inventory menu slots: 9 to 35 are the main inventory, 36 to 44 the
	// hotbar and 45 the off hand.
	inventorySize = 46
	hotbarStart   = 36
	hotbarSize    = 9
	offHandSlot   = 45

	// storageSlots is the size of the main inventory and hotbar together.
	storageSlots = 36

	// maxStackSize is the largest stack the max_stack_size component allows.
	maxStackSize = 99

	// inventoryStateID is sent as the inventory menu's revision. Clients only
	// echo it in click packets, which the server ignores.
	inventoryStateID = 0
)

var (
	itemRegistry          = registry.MustGet(registry.Item)
	enchantmentsComponent = registry.MustGet(registry.DataComponentType).MustID("minecraft:enchantments")
)

// menuSlot converts an index of vanilla's Inventory item list, which starts
// with the hotbar and continues with the main inventory, to a menu slot.
func menuSlot(i int) int {
	if i < hotbarSize {
		return hotbarStart + i
	}

	return i
}

func (p *Player) setHeldSlot(slot int16) {
	if slot < 0 || slot >= hotbarSize {
		return
	}

	p.mu.Lock()
	p.heldSlot = int(slot)
	p.mu.Unlock()
}

// setCreativeSlot records a stack the player put in a slot from the creative
// inventory.
func (p *Player) setCreativeSlot(slot int16, stack packet.ItemStack) {
	if p.GameMode != packet.GameModeCreative || slot < 0 || slot >= inventorySize || stack.Count > maxStackSize {
		return
	}

	if stack.IsEmpty() || stack.Item <= 0 || int(stack.Item) >= len(itemRegistry.Entries) {
		stack = packet.ItemStack{}
	}

	p.mu.Lock()
	p.inventory[slot] = stack
	p.mu.Unlock()
}

// heldItem returns the item in a hand, or 0 for an empty hand.
func (p *Player) heldItem(hand int32) int32 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if hand == packet.HandOff {
		return p.inventory[offHandSlot].Item
	}

	return p.inventory[hotbarStart+p.heldSlot].Item
}

// dropHeldItem removes the whole held stack or one of its items, as the client
// already did when the player pressed the drop key. Items aren't dropped into
// the world.
func (p *Player) dropHeldItem(all bool) {
	if p.GameMode == packet.GameModeSpectator {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	stack := &p.inventory[hotbarStart+p.heldSlot]

	if all || stack.Count <= 1 {
		*stack = packet.ItemStack{}
	} else {
		stack.Count--
	}
}

// handlePickItemFromBlock gives the player the item of the block it
// middle-clicked. Ctrl-picking copies nothing extra, since the server keeps no
// block entity data.
func (s *Server) handlePickItemFromBlock(p *Player, pk *packet.PickItemFromBlock) {
	if !p.snapshot().inReach(pk.Pos) {
		return
	}

	state, loaded := s.world.Block(int(pk.Pos.X), int(pk.Pos.Y), int(pk.Pos.Z))

	if !loaded {
		return
	}

	name, ok := block.PickItem(state)

	if !ok {
		return
	}

	if item, ok := itemRegistry.ID(name); ok {
		p.pickItem(item)
	}
}

// pickItem moves item into the player's hand and sends the changed slots.
func (p *Player) pickItem(item int32) {
	p.mu.Lock()
	changed := p.selectItem(item)
	updates := []packet.Packet{&packet.SetHeldSlot{Slot: int32(p.heldSlot)}}

	for _, slot := range changed {
		updates = append(updates, &packet.ContainerSetSlot{
			ContainerID: packet.InventoryContainer,
			StateID:     inventoryStateID,
			Slot:        int16(slot),
			Stack:       p.inventory[slot],
		})
	}

	p.mu.Unlock()

	for _, u := range updates {
		p.send(u)
	}
}

// selectItem selects item like vanilla's tryPickItem: a stack of it in the
// hotbar is selected, one elsewhere in the inventory is swapped into the
// hotbar, and otherwise creative players get one. It returns the menu slots
// that changed. p.mu must be held.
func (p *Player) selectItem(item int32) []int {
	switch slot := p.findStack(item); {
	case slot >= hotbarStart:
		p.heldSlot = slot - hotbarStart

		return nil
	case slot >= 0:
		p.heldSlot = p.suitableHotbarSlot()
		held := hotbarStart + p.heldSlot
		p.inventory[held], p.inventory[slot] = p.inventory[slot], p.inventory[held]

		return []int{held, slot}
	case p.GameMode == packet.GameModeCreative:
		p.heldSlot = p.suitableHotbarSlot()
		held := hotbarStart + p.heldSlot
		changed := []int{held}

		// The replaced stack moves to a free slot if there is one.
		if free := p.freeSlot(); free >= 0 && !p.inventory[held].IsEmpty() {
			p.inventory[free] = p.inventory[held]
			changed = append(changed, free)
		}

		p.inventory[held] = packet.ItemStack{Count: 1, Item: item}

		return changed
	}

	return nil
}

// findStack returns the first menu slot, hotbar first, holding item without
// any components, or -1.
func (p *Player) findStack(item int32) int {
	for i := range storageSlots {
		s := &p.inventory[menuSlot(i)]

		if !s.IsEmpty() && s.Item == item && len(s.Components) == 0 && len(s.Removed) == 0 {
			return menuSlot(i)
		}
	}

	return -1
}

// suitableHotbarSlot returns the hotbar slot a picked item goes to: the first
// empty one from the selected slot onwards, else the first without an
// enchanted item, else the selected one.
func (p *Player) suitableHotbarSlot() int {
	for i := range hotbarSize {
		if slot := (p.heldSlot + i) % hotbarSize; p.inventory[hotbarStart+slot].IsEmpty() {
			return slot
		}
	}

	for i := range hotbarSize {
		if slot := (p.heldSlot + i) % hotbarSize; !isEnchanted(&p.inventory[hotbarStart+slot]) {
			return slot
		}
	}

	return p.heldSlot
}

// freeSlot returns the first empty menu slot, hotbar first, or -1.
func (p *Player) freeSlot() int {
	for i := range storageSlots {
		if p.inventory[menuSlot(i)].IsEmpty() {
			return menuSlot(i)
		}
	}

	return -1
}

// isEnchanted reports whether a stack has an enchantments component with any
// entries. Its value starts with the VarInt entry count, which encodes as a
// zero byte only when it is zero.
func isEnchanted(s *packet.ItemStack) bool {
	for _, c := range s.Components {
		if c.Type == enchantmentsComponent {
			return len(c.Data) > 0 && c.Data[0] != 0
		}
	}

	return false
}

// inventoryPackets returns the packets that show the player its inventory.
func (p *Player) inventoryPackets() []packet.Packet {
	p.mu.Lock()
	defer p.mu.Unlock()

	return []packet.Packet{
		&packet.SetHeldSlot{Slot: int32(p.heldSlot)},
		&packet.ContainerSetContent{
			ContainerID: packet.InventoryContainer,
			StateID:     inventoryStateID,
			Items:       append([]packet.ItemStack(nil), p.inventory[:]...),
		},
	}
}

// loadInventory restores a saved inventory, skipping items that no longer
// exist.
func (p *Player) loadInventory(saved storage.PlayerData) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if saved.HeldSlot >= 0 && saved.HeldSlot < hotbarSize {
		p.heldSlot = saved.HeldSlot
	}

	for _, item := range saved.Inventory {
		id, ok := itemRegistry.ID(item.Item)

		if !ok || id == 0 || item.Slot < 0 || item.Slot >= inventorySize || item.Count <= 0 {
			continue
		}

		stack := packet.ItemStack{Count: item.Count, Item: id, Removed: item.Removed}

		for _, c := range item.Components {
			stack.Components = append(stack.Components, packet.ItemComponent(c))
		}

		p.inventory[item.Slot] = stack
	}
}

// savedInventory returns the non-empty slots in their saved form. p.mu must be
// held.
func (p *Player) savedInventory() []storage.InventoryItem {
	var items []storage.InventoryItem

	for slot, stack := range p.inventory {
		if stack.IsEmpty() {
			continue
		}

		item := storage.InventoryItem{Slot: slot, Item: itemRegistry.Entries[stack.Item], Count: stack.Count, Removed: stack.Removed}

		for _, c := range stack.Components {
			item.Components = append(item.Components, storage.ItemComponent(c))
		}

		items = append(items, item)
	}

	return items
}
