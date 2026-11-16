package server

import (
	"math"
	"math/rand/v2"
	"sync"

	"main/src/entity"
	"main/src/item"
	"main/src/packet"
	"main/src/physics"
)

// itemEntities holds the dropped items in the world. Items may be dropped
// from any goroutine; the game loop moves them into items and simulates them.
type itemEntities struct {
	mu      sync.Mutex
	pending []*entity.Item

	items []*entity.Item
	rand  *rand.Rand
}

// registerItemEntities simulates dropped items every tick: they fall, slide,
// merge and despawn, and players walking over them pick them up. Items in
// unloaded chunks wait until their chunk loads again.
func (s *Server) registerItemEntities() {
	s.items.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	s.ticks.every(1, func(t *tick) {
		s.items.mu.Lock()
		s.items.items = append(s.items.items, s.items.pending...)
		s.items.pending = nil
		s.items.mu.Unlock()

		for _, it := range s.items.items {
			if s.isLoaded(it.Position()) && it.Tick(s.world, s.items.rand) {
				it.MergeNearby(s.items.items)
			}
		}

		s.pickUpItems(t.players)

		live := s.items.items[:0]

		for _, it := range s.items.items {
			if !it.Removed {
				live = append(live, it)
			}
		}

		clear(s.items.items[len(live):])
		s.items.items = live
	})

	s.trackEntities(func() []entity.Entity {
		out := make([]entity.Entity, len(s.items.items))

		for i, it := range s.items.items {
			out[i] = it
		}

		return out
	})
}

// isLoaded reports whether the chunk a position is in is loaded.
func (s *Server) isLoaded(pos physics.Vec3) bool {
	_, loaded := s.world.Block(int(math.Floor(pos.X)), int(math.Floor(pos.Y)), int(math.Floor(pos.Z)))

	return loaded
}

// spawnItem drops a stack into the world at a position, moving at a velocity
// in blocks per tick, that can't be picked up for pickupDelay ticks.
func (s *Server) spawnItem(stack item.Stack, pos, vel physics.Vec3, pickupDelay int32) {
	if stack.IsEmpty() {
		return
	}

	it := entity.NewItem(s.nextEntityID.Add(1), stack, pos, vel, pickupDelay)

	s.items.mu.Lock()
	s.items.pending = append(s.items.pending, it)
	s.items.mu.Unlock()
}

// dropFromPlayer throws a stack from just below a player's eyes the way it is
// looking, like vanilla's drop key.
func (s *Server) dropFromPlayer(p *Player, stack item.Stack) {
	snap := p.snapshot()
	eyes := eyeHeight

	if snap.crouching() {
		eyes = crouchingEyeHeight
	}

	yaw, pitch := float64(snap.Yaw)*math.Pi/180, float64(snap.Pitch)*math.Pi/180
	spread, angle := 0.02*rand.Float64(), rand.Float64()*2*math.Pi

	vel := physics.Vec3{
		X: -math.Sin(yaw)*math.Cos(pitch)*0.3 + math.Cos(angle)*spread,
		Y: -math.Sin(pitch)*0.3 + 0.1 + (rand.Float64()-rand.Float64())*0.1,
		Z: math.Cos(yaw)*math.Cos(pitch)*0.3 + math.Sin(angle)*spread,
	}

	s.spawnItem(stack, physics.Vec3{X: snap.X, Y: snap.Y + eyes - 0.3, Z: snap.Z}, vel, entity.PlayerDropPickupDelay)
}

// dropFromBlock pops a stack out near the middle of a block, like vanilla's
// broken blocks.
func (s *Server) dropFromBlock(pos packet.BlockPos, stack item.Stack) {
	jitter := func() float64 { return rand.Float64()*0.5 - 0.25 }
	at := physics.Vec3{
		X: float64(pos.X) + 0.5 + jitter(),
		Y: float64(pos.Y) + 0.5 + jitter() - 0.125,
		Z: float64(pos.Z) + 0.5 + jitter(),
	}

	vel := physics.Vec3{X: rand.Float64()*0.2 - 0.1, Y: 0.2, Z: rand.Float64()*0.2 - 0.1}

	s.spawnItem(stack, at, vel, entity.BlockDropPickupDelay)
}

// pickUpItems gives players the items within a block of them horizontally
// and half a block vertically. Spectators pick nothing up.
func (s *Server) pickUpItems(players []*Player) {
	for _, p := range players {
		if p.GameMode() == packet.GameModeSpectator {
			continue
		}

		reach := p.snapshot().box().Inflate(1, 0.5, 1)

		for _, it := range s.items.items {
			if it.Pickable() && reach.Intersects(it.Box()) {
				s.pickUp(p, it)
			}
		}
	}
}

// pickUp moves as much of a dropped stack into a player's inventory as fits,
// showing everyone who sees the item fly to the player.
func (s *Server) pickUp(p *Player, it *entity.Item) {
	before := it.Stack.Count

	p.mu.Lock()
	rest, changed := p.inventory.Add(it.Stack, p.GameMode() == packet.GameModeCreative)
	p.mu.Unlock()

	if rest.Count == before {
		return
	}

	s.entities.SendToViewers(it.ID(), &packet.TakeItemEntity{ItemID: it.ID(), CollectorID: p.EntityID, Amount: before})

	it.Stack = rest
	it.Removed = rest.IsEmpty()

	p.sendSlots(changed)
}
