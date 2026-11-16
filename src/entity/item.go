package entity

import (
	"math"
	"math/rand/v2"

	"main/src/block"
	"main/src/item"
	"main/src/packet"
	"main/src/physics"
	"main/src/registry"
	"main/src/uuid"
)

// Pickup delays of dropped items, in ticks.
const (
	PlayerDropPickupDelay = 40
	BlockDropPickupDelay  = 10
)

const (
	itemSize     = 0.25
	itemGravity  = 0.04
	itemDrag     = 0.98
	itemDespawn  = 6000
	itemMaxMerge = 64

	// fluidDepth is how deep in a fluid an item must be to float in it.
	fluidDepth = 0.1
)

var itemEntityType = registry.MustGet(registry.EntityType).MustID("minecraft:item")

// Item is a dropped item stack that falls, slides and floats like vanilla's
// item entities until it is picked up, merges with another or despawns. It
// isn't safe for concurrent use.
type Item struct {
	id   int32
	uuid uuid.UUID

	// Stack is the dropped stack. Removed items are gone from the world.
	Stack   item.Stack
	Removed bool

	pos, vel    physics.Vec3
	onGround    bool
	pickupDelay int32
	age         int32
	ticks       int32
	impulse     bool
}

// NewItem returns a dropped stack at a position, moving at a velocity in
// blocks per tick, that can't be picked up for pickupDelay ticks.
func NewItem(id int32, stack item.Stack, pos, vel physics.Vec3, pickupDelay int32) *Item {
	return &Item{id: id, uuid: uuid.New(), Stack: stack, pos: pos, vel: vel, pickupDelay: pickupDelay}
}

// ID implements Entity.
func (it *Item) ID() int32 { return it.id }

// UUID implements Entity.
func (it *Item) UUID() uuid.UUID { return it.uuid }

// Type implements Entity.
func (it *Item) Type() int32 { return itemEntityType }

// State implements Entity.
func (it *Item) State() State {
	return State{
		X: it.pos.X, Y: it.pos.Y, Z: it.pos.Z,
		VX: it.vel.X, VY: it.vel.Y, VZ: it.vel.Z,
		OnGround: it.onGround,
		Impulse:  it.impulse,
		Metadata: []packet.EntityDataValue{{Index: packet.DataItem, Type: packet.DataTypeItemStack, Item: it.Stack}},
	}
}

// Position returns where the item is.
func (it *Item) Position() physics.Vec3 {
	return it.pos
}

// Box returns the item's bounding box.
func (it *Item) Box() physics.AABB {
	return physics.Box(it.pos, itemSize, itemSize)
}

// Pickable reports whether players may pick the item up.
func (it *Item) Pickable() bool {
	return !it.Removed && it.pickupDelay == 0
}

// Tick advances the item by one game tick and reports whether it should try
// merging with nearby items: every 2 ticks while it moves between blocks and
// every 40 otherwise.
func (it *Item) Tick(w physics.Blocks, r *rand.Rand) bool {
	if it.Stack.IsEmpty() {
		it.Removed = true
		return false
	}

	it.ticks++

	if it.pickupDelay > 0 {
		it.pickupDelay--
	}

	start, before := it.pos, it.vel

	switch fluid, inFluid := it.fluid(w); {
	case inFluid && !block.IsLava(fluid):
		it.floatIn(0.99)
	case inFluid:
		it.floatIn(0.95)
	default:
		it.vel.Y -= itemGravity
	}

	box := it.Box()
	stuck := physics.Collides(w, box.Inflate(-1e-7, -1e-7, -1e-7))

	if stuck {
		it.moveTowardsClosestSpace(w, r, (box.Min.Y+box.Max.Y)/2)
	}

	if !it.onGround || it.vel.HorizontalLengthSqr() > 1e-5 || (it.ticks+it.id)%4 == 0 {
		it.move(w, stuck)

		friction := float32(itemDrag)

		if it.onGround {
			friction = blockFriction(w, it.pos.X, it.pos.Y-0.500001, it.pos.Z) * itemDrag
		}

		it.vel.X *= float64(friction)
		it.vel.Y *= itemDrag
		it.vel.Z *= float64(friction)

		if it.onGround && it.vel.Y < 0 {
			it.vel.Y *= -0.5
		}
	}

	it.age++
	it.impulse = it.vel.Sub(before).LengthSqr() > 0.01
	it.Removed = it.age >= itemDespawn

	interval := int32(40)

	if blockOf(start) != blockOf(it.pos) {
		interval = 2
	}

	return it.ticks%interval == 0
}

// fluid returns the fluid the item is submerged in, if any.
func (it *Item) fluid(w physics.Blocks) (block.StateID, bool) {
	s, loaded := w.Block(floor(it.pos.X), floor(it.pos.Y+fluidDepth), floor(it.pos.Z))

	return s, loaded && block.HasFluid(s)
}

// floatIn slows the item in a fluid and lets it rise slowly.
func (it *Item) floatIn(drag float64) {
	it.vel.X *= drag
	it.vel.Z *= drag

	if it.vel.Y < 0.06 {
		it.vel.Y += 5e-4
	}
}

// move moves the item by its velocity, stopping at blocks unless it is stuck
// inside one.
func (it *Item) move(w physics.Blocks, stuck bool) {
	if stuck {
		it.pos = it.pos.Add(it.vel)
		return
	}

	moved := physics.Move(w, it.Box(), it.vel)
	it.pos = it.pos.Add(moved)

	vertical := !nearlyEqual(moved.Y, it.vel.Y)
	it.onGround = vertical && it.vel.Y < 0

	if !nearlyEqual(moved.X, it.vel.X) {
		it.vel.X = 0
	}

	if !nearlyEqual(moved.Z, it.vel.Z) {
		it.vel.Z = 0
	}

	if !vertical {
		return
	}

	// Slime blocks bounce falling items back up; other blocks stop them.
	if on, _ := w.Block(floor(it.pos.X), floor(it.pos.Y-0.2), floor(it.pos.Z)); block.ByState(on).Name == "minecraft:slime_block" {
		if it.vel.Y < 0 {
			it.vel.Y *= -0.8
		}

		return
	}

	it.vel.Y = 0
}

// moveTowardsClosestSpace pushes an item stuck inside a block towards the
// nearest open side, or up.
func (it *Item) moveTowardsClosestSpace(w physics.Blocks, r *rand.Rand, centerY float64) {
	bx, by, bz := floor(it.pos.X), floor(centerY), floor(it.pos.Z)
	frac := physics.Vec3{X: it.pos.X - float64(bx), Y: centerY - float64(by), Z: it.pos.Z - float64(bz)}

	sides := []struct {
		dx, dy, dz int
		distance   float64
	}{
		{0, 0, -1, frac.Z},
		{0, 0, 1, 1 - frac.Z},
		{-1, 0, 0, frac.X},
		{1, 0, 0, 1 - frac.X},
		{0, 1, 0, 1 - frac.Y},
	}

	best, closest := sides[4], math.MaxFloat64

	for _, side := range sides {
		if !physics.IsSolid(w, bx+side.dx, by+side.dy, bz+side.dz) && side.distance < closest {
			best, closest = side, side.distance
		}
	}

	speed := float64(r.Float32()*0.2 + 0.1)
	it.vel = physics.Vec3{X: it.vel.X * 0.75, Y: it.vel.Y * 0.75, Z: it.vel.Z * 0.75}

	switch {
	case best.dx != 0:
		it.vel.X = float64(best.dx) * speed
	case best.dy != 0:
		it.vel.Y = float64(best.dy) * speed
	default:
		it.vel.Z = float64(best.dz) * speed
	}
}

// MergeNearby merges the item with items within half a block of it
// horizontally. Like vanilla, two stacks only merge when they fit in one,
// and the larger absorbs the smaller.
func (it *Item) MergeNearby(items []*Item) {
	if !it.mergeable() {
		return
	}

	area := it.Box().Inflate(0.5, 0, 0.5)

	for _, other := range items {
		if other == it || !other.mergeable() || !area.Intersects(other.Box()) {
			continue
		}

		if it.tryMerge(other); it.Removed {
			return
		}
	}
}

func (it *Item) mergeable() bool {
	return !it.Removed && it.age < itemDespawn && it.Stack.Count < it.Stack.MaxStackSize()
}

func (it *Item) tryMerge(other *Item) {
	a, b := it.Stack, other.Stack

	if a.Count+b.Count > b.MaxStackSize() || !item.SameItemSameComponents(a, b) {
		return
	}

	if b.Count < a.Count {
		it.absorb(other)
	} else {
		other.absorb(it)
	}
}

func (it *Item) absorb(origin *Item) {
	n := min(min(it.Stack.MaxStackSize(), itemMaxMerge)-it.Stack.Count, origin.Stack.Count)
	it.Stack.Count += n
	origin.Stack.Count -= n
	it.pickupDelay = max(it.pickupDelay, origin.pickupDelay)
	it.age = min(it.age, origin.age)
	origin.Removed = origin.Stack.IsEmpty()
}

// blockFriction returns the friction of the block at a position. Unloaded
// blocks read as air, which has the friction of most blocks.
func blockFriction(w physics.Blocks, x, y, z float64) float32 {
	s, _ := w.Block(floor(x), floor(y), floor(z))

	return block.Friction(s)
}

func blockOf(v physics.Vec3) [3]int {
	return [3]int{floor(v.X), floor(v.Y), floor(v.Z)}
}

func floor(v float64) int {
	return int(math.Floor(v))
}

// nearlyEqual compares like vanilla's Mth.equal.
func nearlyEqual(a, b float64) bool {
	return math.Abs(b-a) < 1e-5
}
