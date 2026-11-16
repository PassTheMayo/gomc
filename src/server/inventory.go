package server

import (
	"time"

	"main/src/block"
	"main/src/item"
	"main/src/packet"
	"main/src/registry"
)

var itemRegistry = registry.MustGet(registry.Item)

// registerInventory tracks players' inventories and selected hotbar slots,
// handles clicks in the inventory window, block picking, dropping items and
// swapping hands, and sends joining players their inventory. Players leaving
// put their cursor and crafting grid back, like closing the window.
func (s *Server) registerInventory() {
	r := &s.router

	r.onJoin(func(p *Player) {
		p.mu.Lock()
		held := p.inventory.Selected()
		p.mu.Unlock()

		p.Send(&packet.SetHeldSlot{Slot: int32(held)})
		p.sendInventory()
	})

	r.onLeave(s.closeInventory)

	on(r, func(p *Player, pk *packet.SetCarriedItem) {
		p.mu.Lock()
		p.inventory.Select(int(pk.Slot))
		p.mu.Unlock()
	})

	on(r, s.handleCreativeSlot)
	on(r, s.handlePickItemFromBlock)
	on(r, s.handleContainerClick)

	on(r, func(p *Player, pk *packet.ContainerClose) {
		if pk.ContainerID == packet.InventoryContainer {
			s.closeInventory(p)
		}
	})

	on(r, func(p *Player, pk *packet.PlayerAction) {
		if p.GameMode() == packet.GameModeSpectator {
			return
		}

		switch pk.Action {
		// The client already removed what the player dropped.
		case packet.ActionDropItem, packet.ActionDropAllItems:
			p.mu.Lock()
			dropped := p.inventory.DropSelected(pk.Action == packet.ActionDropAllItems)
			p.mu.Unlock()

			s.dropFromPlayer(p, dropped)
		// The client waits for the server to swap its hands.
		case packet.ActionSwapWithOffHand:
			p.mu.Lock()
			changed := p.inventory.SwapHands()
			p.mu.Unlock()

			p.sendSlots(changed)
		}
	})
}

// sendInventory sends the client every slot of its inventory window and the
// stack on its cursor.
func (p *Player) sendInventory() {
	p.mu.Lock()
	pk := &packet.ContainerSetContent{
		ContainerID: packet.InventoryContainer,
		StateID:     p.inventory.NextRevision(),
		Items:       p.inventory.Menu(),
		Carried:     p.inventory.Carried(),
	}
	p.mu.Unlock()

	p.Send(pk)
}

// sendSlots sends the client the current stacks of inventory menu slots.
func (p *Player) sendSlots(slots []int) {
	p.mu.Lock()
	menu := p.inventory.Menu()
	packets := make([]*packet.ContainerSetSlot, len(slots))

	for i, slot := range slots {
		packets[i] = &packet.ContainerSetSlot{
			ContainerID: packet.InventoryContainer,
			StateID:     p.inventory.NextRevision(),
			Slot:        int16(slot),
			Stack:       menu[slot],
		}
	}

	p.mu.Unlock()

	for _, pk := range packets {
		p.Send(pk)
	}
}

// handleCreativeSlot records a stack a creative player put in a menu slot,
// or drops it when the slot is outside the window. Drops are throttled like
// vanilla's to stop clients flooding the world with items.
func (s *Server) handleCreativeSlot(p *Player, pk *packet.SetCreativeModeSlot) {
	stack := pk.Stack

	if p.GameMode() != packet.GameModeCreative || stack.Count > stack.MaxStackSize() {
		return
	}

	if stack.Item <= 0 || int(stack.Item) >= len(itemRegistry.Entries) {
		stack = item.Stack{}
	}

	p.mu.Lock()
	drop := pk.Slot < 0 && !stack.IsEmpty() && p.dropThrottle.allow(time.Now())

	if pk.Slot >= 0 {
		p.inventory.SetMenuSlot(int(pk.Slot), stack)
	}

	p.mu.Unlock()

	if drop {
		s.dropFromPlayer(p, stack)
	}
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
