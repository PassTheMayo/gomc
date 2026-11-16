package server

import (
	"main/src/block"
	"main/src/packet"
	"main/src/world"
)

const (
	// maxReach is the furthest block a player may change, from its eyes.
	// Vanilla allows 4.5 blocks in survival and 5 in creative; the rest is a
	// margin for latency.
	maxReach = 7.0

	eyeHeight    = 1.62
	playerWidth  = 0.6
	playerHeight = 1.8
)

var (
	airState   = block.MustParse("air")
	waterState = block.MustParse("water[level=0]")
)

// canEditBlocks reports whether the player's game mode lets it change blocks.
func (p *Player) canEditBlocks() bool {
	return p.GameMode == packet.GameModeSurvival || p.GameMode == packet.GameModeCreative
}

// inReach reports whether the player can reach the center of a block.
func (s snapshot) inReach(pos packet.BlockPos) bool {
	dx := float64(pos.X) + 0.5 - s.X
	dy := float64(pos.Y) + 0.5 - (s.Y + eyeHeight)
	dz := float64(pos.Z) + 0.5 - s.Z

	return dx*dx+dy*dy+dz*dz <= maxReach*maxReach
}

// overlaps reports whether the player's body overlaps a block.
func (s snapshot) overlaps(pos packet.BlockPos) bool {
	bx, by, bz := float64(pos.X), float64(pos.Y), float64(pos.Z)
	half := playerWidth / 2

	return s.X+half > bx && s.X-half < bx+1 && s.Y+playerHeight > by && s.Y < by+1 && s.Z+half > bz && s.Z-half < bz+1
}

// handleDigging breaks blocks. Creative players break instantly; survival
// players report when they finish digging, except for plants and other
// blocks that break on the first hit.
func (s *Server) handleDigging(p *Player, pk *packet.PlayerAction) {
	defer p.send(&packet.BlockChangedAck{Sequence: pk.Sequence})

	current, loaded := s.world.Block(int(pk.Pos.X), int(pk.Pos.Y), int(pk.Pos.Z))

	switch {
	case !loaded || !p.canEditBlocks() || !p.snapshot().inReach(pk.Pos):
		return
	case pk.Action == packet.ActionStopDestroyBlock && p.GameMode == packet.GameModeSurvival:
	case pk.Action == packet.ActionStartDestroyBlock && (p.GameMode == packet.GameModeCreative || breaksInstantly(current)):
	default:
		return
	}

	s.destroyBlock(pk.Pos, current)
}

// breaksInstantly reports whether survival players break a block with one hit.
func breaksInstantly(s block.StateID) bool {
	return !block.IsAir(s) && !block.IsSolid(s) && !block.HasFluid(s) && block.LightOpacity(s) == 0
}

// destroyBlock removes a block, leaving water behind if it was waterlogged,
// along with the other half of two block objects like doors and beds.
func (s *Server) destroyBlock(pos packet.BlockPos, current block.StateID) {
	if block.IsAir(current) {
		return
	}

	s.world.SetBlock(int(pos.X), int(pos.Y), int(pos.Z), removedState(current))

	if dx, dy, dz, ok := block.LinkedPart(current); ok {
		x, y, z := int(pos.X)+dx, int(pos.Y)+dy, int(pos.Z)+dz

		if other, loaded := s.world.Block(x, y, z); loaded && block.ByState(other) == block.ByState(current) {
			s.world.SetBlock(x, y, z, removedState(other))
		}
	}
}

func removedState(s block.StateID) block.StateID {
	if block.HasFluid(s) && block.IsSolid(s) {
		return waterState
	}

	return airState
}

// handleUseItemOn places the block item held in the used hand against the
// clicked face.
func (s *Server) handleUseItemOn(p *Player, pk *packet.UseItemOn) {
	defer p.send(&packet.BlockChangedAck{Sequence: pk.Sequence})

	if pk.Face < int32(block.Down) || pk.Face > int32(block.East) || !p.canEditBlocks() {
		return
	}

	item := p.heldItem(pk.Hand)

	if item == 0 || int(item) >= len(itemRegistry.Entries) {
		return
	}

	b, isBlock := block.ForItem(itemRegistry.Entries[item])
	clicked, loaded := s.world.Block(int(pk.Pos.X), int(pk.Pos.Y), int(pk.Pos.Z))

	if !isBlock || !loaded {
		return
	}

	face := block.Direction(pk.Face)
	target := pk.Pos

	if !block.IsReplaceable(clicked) {
		dx, dy, dz := face.Offset()
		target = packet.BlockPos{X: pk.Pos.X + int32(dx), Y: pk.Pos.Y + int32(dy), Z: pk.Pos.Z + int32(dz)}
	}

	snap := p.snapshot()

	if !snap.inReach(target) {
		return
	}

	replacing, _ := s.world.Block(int(target.X), int(target.Y), int(target.Z))
	placements := b.Place(block.PlaceContext{Face: face, CursorY: pk.CursorY, Yaw: snap.Yaw, Pitch: snap.Pitch, Replacing: replacing})
	others := s.joinedPlayers()

	// Every part must fit before any is placed.
	for _, pl := range placements {
		pos := packet.BlockPos{X: target.X + int32(pl.DX), Y: target.Y + int32(pl.DY), Z: target.Z + int32(pl.DZ)}

		if !s.canPlaceAt(pos, pl.State, others) {
			return
		}
	}

	for _, pl := range placements {
		s.world.SetBlock(int(target.X)+pl.DX, int(target.Y)+pl.DY, int(target.Z)+pl.DZ, pl.State)
	}
}

// canPlaceAt reports whether state may be placed at pos: inside the world, in
// a loaded chunk, into a replaceable block and not inside a player.
func (s *Server) canPlaceAt(pos packet.BlockPos, state block.StateID, players []*Player) bool {
	if int(pos.Y) < s.world.Dim.MinY || int(pos.Y) > s.world.Dim.MaxY() {
		return false
	}

	current, loaded := s.world.Block(int(pos.X), int(pos.Y), int(pos.Z))

	if !loaded || !block.IsReplaceable(current) {
		return false
	}

	if block.IsSolid(state) {
		for _, other := range players {
			if other.snapshot().overlaps(pos) {
				return false
			}
		}
	}

	return true
}

// broadcastBlockChange sends a changed block to every player whose view
// includes its chunk. It runs while the chunk is locked; see World.BlockChanged.
func (s *Server) broadcastBlockChange(x, y, z int, state block.StateID) {
	chunk := world.ChunkPos{X: int32(x >> 4), Z: int32(z >> 4)}
	update := &packet.BlockUpdate{Pos: packet.BlockPos{X: int32(x), Y: int32(y), Z: int32(z)}, State: int32(state)}

	for _, p := range s.joinedPlayers() {
		snap := p.snapshot()

		if inView(snap.chunkPos(), snap.ViewDistance, chunk) {
			p.send(update)
		}
	}
}
