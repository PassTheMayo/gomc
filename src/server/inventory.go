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
// handles block picking, drops and swapping hands, and sends joining players
// their inventory.
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

	on(r, func(p *Player, pk *packet.PlayerAction) {
		if p.GameMode() == packet.GameModeSpectator {
			return
		}

		switch pk.Action {
		// The client already removed what the player dropped. Items aren't
		// dropped into the world.
		case packet.ActionDropItem, packet.ActionDropAllItems:
			p.mu.Lock()
			p.inventory.DropSelected(pk.Action == packet.ActionDropAllItems)
			p.mu.Unlock()
		// The client waits for the server to swap its hands.
		case packet.ActionSwapWithOffHand:
			p.mu.Lock()
			changed := p.inventory.SwapHands()
			p.mu.Unlock()

			p.sendSlots(changed)
		}
	})
}

// sendSlots sends the client the current stacks of inventory menu slots.
func (p *Player) sendSlots(slots []int) {
	p.mu.Lock()
	menu := p.inventory.Menu()
	p.mu.Unlock()

	for _, slot := range slots {
		p.Send(&packet.ContainerSetSlot{
			ContainerID: packet.InventoryContainer,
			StateID:     inventoryStateID,
			Slot:        int16(slot),
			Stack:       menu[slot],
		})
	}
}

// setCreativeSlot records a stack the player put in a menu slot from the
// creative inventory.
func (p *Player) setCreativeSlot(slot int, stack item.Stack) {
	if p.GameMode() != packet.GameModeCreative || stack.Count > maxStackSize {
		return
	}

	if stack.Item <= 0 || int(stack.Item) >= len(itemRegistry.Entries) {
		stack = item.Stack{}
	}

	p.mu.Lock()
	p.inventory.SetMenuSlot(slot, stack)
	p.mu.Unlock()
}

// heldStack returns the stack in a hand.
func (p *Player) heldStack(hand int32) item.Stack {
	p.mu.Lock()
	defer p.mu.Unlock()

	if hand == packet.HandOff {
		return p.inventory.OffHand()
	}

	return p.inventory.MainHand()
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
	changed := p.inventory.Pick(item.Of(id, 1), p.GameMode() == packet.GameModeCreative)
	held := p.inventory.Selected()
	p.mu.Unlock()

	p.Send(&packet.SetHeldSlot{Slot: int32(held)})
	p.sendSlots(changed)
}
