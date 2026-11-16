package entity

import (
	"math/rand/v2"
	"testing"

	"main/src/block"
	"main/src/item"
	"main/src/physics"
)

// ground is a world of air above a stone floor whose top is at y = 64.
type ground struct {
	surface block.StateID
}

func (g ground) Block(x, y, z int) (block.StateID, bool) {
	switch {
	case y < 63:
		return block.MustParse("stone"), true
	case y == 63:
		return g.surface, true
	}

	return block.MustParse("air"), true
}

var stoneFloor = ground{surface: block.MustParse("stone")}

func newRand() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func TestItemFallsAndRests(t *testing.T) {
	it := NewItem(1, item.Of(1, 1), physics.Vec3{X: 0.5, Y: 70, Z: 0.5}, physics.Vec3{X: 0.1, Y: 0.2}, BlockDropPickupDelay)
	r := newRand()

	for range 200 {
		it.Tick(stoneFloor, r)
	}

	if it.pos.Y != 64 || !it.onGround {
		t.Fatalf("item came to rest at y=%v, on ground %v", it.pos.Y, it.onGround)
	}

	// It drifts about 2 blocks while falling and a little further on the floor.
	if it.vel.HorizontalLengthSqr() > 1e-6 || it.pos.X < 2.2 || it.pos.X > 3 {
		t.Fatalf("item slid to x=%v with velocity %+v", it.pos.X, it.vel)
	}

	if !it.Pickable() {
		t.Fatal("the pickup delay must run out")
	}
}

// Items slide further on ice.
func TestItemSlidesOnIce(t *testing.T) {
	slide := func(w ground) float64 {
		it := NewItem(1, item.Of(1, 1), physics.Vec3{X: 0.5, Y: 64, Z: 0.5}, physics.Vec3{X: 0.3}, 0)
		r := newRand()

		for range 200 {
			it.Tick(w, r)
		}

		return it.pos.X
	}

	if stone, ice := slide(stoneFloor), slide(ground{surface: block.MustParse("ice")}); ice < stone*3 {
		t.Fatalf("an item slid %v blocks on ice and %v on stone", ice, stone)
	}
}

func TestItemDespawns(t *testing.T) {
	it := NewItem(1, item.Of(1, 1), physics.Vec3{X: 0.5, Y: 64, Z: 0.5}, physics.Vec3{}, 0)
	r := newRand()

	for range itemDespawn - 1 {
		it.Tick(stoneFloor, r)
	}

	if it.Removed {
		t.Fatal("items last 5 minutes")
	}

	it.Tick(stoneFloor, r)

	if !it.Removed {
		t.Fatal("items despawn after 5 minutes")
	}
}

func TestItemsMerge(t *testing.T) {
	at := func(id int32, x float64, stack item.Stack) *Item {
		return NewItem(id, stack, physics.Vec3{X: x, Y: 64, Z: 0.5}, physics.Vec3{}, 0)
	}

	a, b := at(1, 0.5, item.Of(1, 10)), at(2, 1.2, item.Of(1, 20))
	a.MergeNearby([]*Item{a, b})

	if !a.Removed || b.Removed || b.Stack.Count != 30 {
		t.Fatalf("the larger stack must absorb the smaller: %d and %d left", a.Stack.Count, b.Stack.Count)
	}

	// Stacks that wouldn't fit in one, other items and distant items stay apart.
	full, other, far := at(3, 1.2, item.Of(1, 40)), at(4, 1.2, item.Of(2, 1)), at(5, 5, item.Of(1, 1))
	b.MergeNearby([]*Item{b, full, other, far})

	if b.Stack.Count != 30 || full.Removed || other.Removed || far.Removed {
		t.Fatalf("unmergeable items merged: %d left", b.Stack.Count)
	}
}

// An item inside a block is pushed out towards the nearest open side.
func TestItemEscapesBlocks(t *testing.T) {
	it := NewItem(1, item.Of(1, 1), physics.Vec3{X: 0.5, Y: 62.5, Z: 0.5}, physics.Vec3{}, 0)
	r := newRand()

	for range 40 {
		it.Tick(stoneFloor, r)
	}

	if it.pos.Y < 64 {
		t.Fatalf("an item buried in stone stayed at y=%v", it.pos.Y)
	}
}
