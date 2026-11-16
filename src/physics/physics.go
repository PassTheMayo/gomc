// Package physics moves entity bounding boxes through the world's blocks.
// Every block with a collision shape is treated as a full cube.
package physics

import (
	"math"

	"main/src/block"
)

// epsilon is how far boxes may overlap before they count as colliding, like
// vanilla's shape collisions.
const epsilon = 1e-7

// Vec3 is a position, offset or velocity.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns v + o.
func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z}
}

// Sub returns v - o.
func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z}
}

// LengthSqr returns the squared length of v.
func (v Vec3) LengthSqr() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

// HorizontalLengthSqr returns the squared length of v ignoring its height.
func (v Vec3) HorizontalLengthSqr() float64 {
	return v.X*v.X + v.Z*v.Z
}

// AABB is an axis-aligned bounding box.
type AABB struct {
	Min, Max Vec3
}

// Box returns the box of an entity standing at a position: width wide,
// centered on it horizontally, and height tall above it.
func Box(pos Vec3, width, height float64) AABB {
	half := width / 2

	return AABB{
		Min: Vec3{pos.X - half, pos.Y, pos.Z - half},
		Max: Vec3{pos.X + half, pos.Y + height, pos.Z + half},
	}
}

// Offset returns the box moved by d.
func (b AABB) Offset(d Vec3) AABB {
	return AABB{Min: b.Min.Add(d), Max: b.Max.Add(d)}
}

// Inflate returns the box grown by the given amounts on every side.
func (b AABB) Inflate(x, y, z float64) AABB {
	d := Vec3{x, y, z}

	return AABB{Min: b.Min.Sub(d), Max: b.Max.Add(d)}
}

// Intersects reports whether two boxes overlap.
func (b AABB) Intersects(o AABB) bool {
	return b.Min.X < o.Max.X && b.Max.X > o.Min.X &&
		b.Min.Y < o.Max.Y && b.Max.Y > o.Min.Y &&
		b.Min.Z < o.Max.Z && b.Max.Z > o.Min.Z
}

// Blocks is the world entities move through. Unloaded blocks are solid, so
// nothing falls out of the loaded world.
type Blocks interface {
	Block(x, y, z int) (block.StateID, bool)
}

// IsSolid reports whether the block at a position stops movement.
func IsSolid(w Blocks, x, y, z int) bool {
	s, loaded := w.Block(x, y, z)

	return !loaded || block.IsSolid(s)
}

// Collides reports whether a box overlaps any solid block.
func Collides(w Blocks, box AABB) bool {
	collides := false

	eachBlock(box, func(x, y, z int) bool {
		collides = IsSolid(w, x, y, z)
		return !collides
	})

	return collides
}

// Move moves a box by motion, stopping it at solid blocks, and returns how
// far it actually moved. Like vanilla, the vertical axis moves first, then
// the horizontal axis with the larger motion.
func Move(w Blocks, box AABB, motion Vec3) Vec3 {
	var moved Vec3

	axes := []int{1, 0, 2}

	if math.Abs(motion.X) < math.Abs(motion.Z) {
		axes = []int{1, 2, 0}
	}

	for _, axis := range axes {
		d := collideAxis(w, box, axis, component(motion, axis))
		offset := Vec3{}
		setComponent(&offset, axis, d)
		setComponent(&moved, axis, d)
		box = box.Offset(offset)
	}

	return moved
}

// collideAxis returns how far a box can move along one axis, up to d.
func collideAxis(w Blocks, box AABB, axis int, d float64) float64 {
	if d == 0 {
		return 0
	}

	swept := box

	if d > 0 {
		setComponent(&swept.Max, axis, component(box.Max, axis)+d)
	} else {
		setComponent(&swept.Min, axis, component(box.Min, axis)+d)
	}

	eachBlock(swept, func(x, y, z int) bool {
		if !IsSolid(w, x, y, z) {
			return true
		}

		lo := component(Vec3{float64(x), float64(y), float64(z)}, axis)

		switch {
		case d > 0 && lo >= component(box.Max, axis)-epsilon:
			d = math.Min(d, lo-component(box.Max, axis))
		case d < 0 && lo+1 <= component(box.Min, axis)+epsilon:
			d = math.Max(d, lo+1-component(box.Min, axis))
		}

		return true
	})

	return d
}

// eachBlock calls visit for every block a box overlaps until it returns
// false.
func eachBlock(box AABB, visit func(x, y, z int) bool) {
	x0, x1 := int(math.Floor(box.Min.X+epsilon)), int(math.Floor(box.Max.X-epsilon))
	y0, y1 := int(math.Floor(box.Min.Y+epsilon)), int(math.Floor(box.Max.Y-epsilon))
	z0, z1 := int(math.Floor(box.Min.Z+epsilon)), int(math.Floor(box.Max.Z-epsilon))

	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				if !visit(x, y, z) {
					return
				}
			}
		}
	}
}

func component(v Vec3, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	}

	return v.Z
}

func setComponent(v *Vec3, axis int, value float64) {
	switch axis {
	case 0:
		v.X = value
	case 1:
		v.Y = value
	default:
		v.Z = value
	}
}
