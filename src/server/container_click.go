package server

import (
	"main/src/item"
	"main/src/packet"
)

// handleContainerClick applies a click in the inventory window. Clients apply
// clicks themselves, so the server only corrects the slots and cursor they
// got wrong, or resends everything if they clicked on an outdated window.
// Spectators can't change their inventory.
func (s *Server) handleContainerClick(p *Player, pk *packet.ContainerClick) {
	if pk.ContainerID != packet.InventoryContainer {
		return
	}

	if p.GameMode() == packet.GameModeSpectator {
		p.sendInventory()
		return
	}

	p.mu.Lock()
	outdated := pk.StateID != p.inventory.Revision()
	before := p.inventory.Menu()
	thrown := p.inventory.Click(int(pk.Slot), int(pk.Button), pk.Mode, p.GameMode() == packet.GameModeCreative)
	after, carried := p.inventory.Menu(), p.inventory.Carried()
	p.mu.Unlock()

	for _, stack := range thrown {
		s.dropFromPlayer(p, stack)
	}

	if outdated {
		p.sendInventory()
		return
	}

	predicted := make(map[int]packet.HashedStack, len(pk.Changed))

	for _, c := range pk.Changed {
		predicted[int(c.Slot)] = c.Stack
	}

	var wrong []int

	for i, stack := range after {
		if h, ok := predicted[i]; ok && !h.Matches(stack) || !ok && !item.Equal(before[i], stack) {
			wrong = append(wrong, i)
		}
	}

	p.sendSlots(wrong)

	if !pk.Carried.Matches(carried) {
		p.Send(&packet.SetCursorItem{Stack: carried})
	}
}

// closeInventory puts the cursor stack and the crafting grid back into the
// player's inventory and drops what doesn't fit.
func (s *Server) closeInventory(p *Player) {
	p.mu.Lock()
	changed, dropped := p.inventory.CloseMenu()
	p.mu.Unlock()

	p.sendSlots(changed)

	for _, stack := range dropped {
		s.dropFromPlayer(p, stack)
	}
}
