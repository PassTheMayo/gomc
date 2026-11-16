package physics

import (
	"math"
	"testing"

	"main/src/block"
)

// floor is a world of air with a stone floor below y = 0 and a stone wall at
// x = 3.
type floor struct{}

var stone, air = block.MustParse("stone"), block.MustParse("air")

func (floor) Block(x, y, z int) (block.StateID, bool) {
	if y < 0 || x == 3 {
		return stone, true
	}

	return air, true
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestMoveLandsOnFloor(t *testing.T) {
	box := Box(Vec3{0.5, 2, 0.5}, 0.25, 0.25)
	moved := Move(floor{}, box, Vec3{0, -5, 0})

	if !near(moved.Y, -2) || moved.X != 0 || moved.Z != 0 {
		t.Fatalf("a box 2 above the floor moved %+v", moved)
	}
}

func TestMoveStopsAtWall(t *testing.T) {
	box := Box(Vec3{2.5, 0, 0.5}, 0.25, 0.25)
	moved := Move(floor{}, box, Vec3{1, 0, 0.5})

	if !near(moved.X, 0.375) || !near(moved.Z, 0.5) {
		t.Fatalf("a box sliding into a wall moved %+v", moved)
	}
}

// A box resting exactly on the floor can still slide along it.
func TestMoveSlidesOnFloor(t *testing.T) {
	box := Box(Vec3{0.5, 0, 0.5}, 0.25, 0.25)
	moved := Move(floor{}, box, Vec3{0.3, -0.1, 0})

	if !near(moved.X, 0.3) || moved.Y != 0 {
		t.Fatalf("a box on the floor moved %+v", moved)
	}
}

func TestCollides(t *testing.T) {
	if Collides(floor{}, Box(Vec3{0.5, 0, 0.5}, 0.25, 0.25)) {
		t.Error("a box resting on the floor doesn't collide with it")
	}

	if !Collides(floor{}, Box(Vec3{0.5, -0.1, 0.5}, 0.25, 0.25)) {
		t.Error("a box sunk into the floor collides")
	}
}
