package server

import (
	"main/src/block"
	"main/src/item"
	"main/src/packet"
	"main/src/registry"
)

const (
	// maxStackSize is the largest stack the max_stack_size component allows.
	maxStackSize = 99

	// inventoryStateID is sent as the inventory menu's revision. Clients only
	// echo it in click packets, which the server ignores.
	inventoryStateID = 0
)

var itemRegistry = registry.MustGet(registry.Item)

// registerInventory tracks players' inventories and selected hotbar slots,
// handles block picking and drops, and sends joining players their inventory.
func (s *Server) registerInventory() {
	r := &s.router

	r.onJoin(func(p *Player) {
		p.mu.Lock()
		held, menu := p.inventory.Selected(), p.inventory.Menu()
		p.mu.Unlock()

		p.Send(&packet.SetHeldSlot{Slot: int32(held)})
		p.Send(&packet.ContainerSetContent{ContainerID: packet.InventoryContainer, StateID: inventoryStateID, Items: menu})
	})

	on(r, func(p *Player, pk *packet.SetCarriedItem) {
		p.mu.Lock()
		p.inventory.Select(int(pk.Slot))
		p.mu.Unlock()
	})

	on(r, func(p *Player, pk *packet.SetCreativeModeSlot) { p.setCreativeSlot(int(pk.Slot), pk.Stack) })
	on(r, s.handlePickItemFromBlock)

	// The client already removed what the player dropped. Items aren't
	// dropped into the world.
	on(r, func(p *Player, pk *packet.PlayerAction) {
		if p.GameMode == packet.GameModeSpectator || (pk.Action != packet.ActionDropItem && pk.Action != packet.ActionDropAllItems) {
			return
		}

		p.mu.Lock()
		p.inventory.DropSelected(pk.Action == packet.ActionDropAllItems)
		p.mu.Unlock()
	})
}

// setCreativeSlot records a stack the player put in a menu slot from the
// creative inventory.
func (p *Player) setCreativeSlot(slot int, stack item.Stack) {
	if p.GameMode != packet.GameModeCreative || stack.Count > maxStackSize {
		return
	}

	if stack.Item <= 0 || int(stack.Item) >= len(itemRegistry.Entries) {
		stack = item.Stack{}
	}

	p.mu.Lock()
	p.inventory.SetMenuSlot(slot, stack)
	p.mu.Unlock()
}

// heldItem returns the item in a hand, or 0 for an empty hand.
func (p *Player) heldItem(hand int32) int32 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if hand == packet.HandOff {
		return p.inventory.OffHand().Item
	}

	return p.inventory.MainHand().Item
}

// handlePickItemFromBlock gives the player the item of the block it
// middle-clicked and sends the slots that changed. Ctrl-picking copies
// nothing extra, since the server keeps no block entity data.
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

	id, ok := itemRegistry.ID(name)

	if !ok {
		return
	}

	p.mu.Lock()
	changed := p.inventory.Pick(item.Of(id, 1), p.GameMode == packet.GameModeCreative)
	updates := []packet.Packet{&packet.SetHeldSlot{Slot: int32(p.inventory.Selected())}}
	menu := p.inventory.Menu()
	p.mu.Unlock()

	for _, slot := range changed {
		updates = append(updates, &packet.ContainerSetSlot{
			ContainerID: packet.InventoryContainer,
			StateID:     inventoryStateID,
			Slot:        int16(slot),
			Stack:       menu[slot],
		})
	}

	for _, u := range updates {
		p.Send(u)
	}
}
