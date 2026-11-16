package block

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// Direction is a block face, numbered like the protocol.
type Direction int32

// Directions.
const (
	Down Direction = iota
	Up
	North
	South
	West
	East
)

var directionNames = [...]string{"down", "up", "north", "south", "west", "east"}

// Name returns the direction as used in block state properties.
func (d Direction) Name() string {
	return directionNames[d]
}

// Offset returns the unit vector the direction points along.
func (d Direction) Offset() (x, y, z int) {
	switch d {
	case Down:
		return 0, -1, 0
	case Up:
		return 0, 1, 0
	case North:
		return 0, 0, -1
	case South:
		return 0, 0, 1
	case West:
		return -1, 0, 0
	}

	return 1, 0, 0
}

// Opposite returns the direction pointing the other way.
func (d Direction) Opposite() Direction {
	return d ^ 1
}

func (d Direction) horizontal() bool {
	return d >= North
}

func (d Direction) axis() string {
	switch d {
	case Down, Up:
		return "y"
	case North, South:
		return "z"
	}

	return "x"
}

func directionNamed(name string) Direction {
	return Direction(slices.Index(directionNames[:], name))
}

// HorizontalFacing returns the horizontal direction a player with yaw looks.
func HorizontalFacing(yaw float32) Direction {
	switch int(math.Floor(float64(yaw)/90+0.5)) & 3 {
	case 0:
		return South
	case 1:
		return West
	case 2:
		return North
	}

	return East
}

// lookDirection returns the direction a player looks, including up and down.
func lookDirection(yaw, pitch float32) Direction {
	switch {
	case pitch >= 45:
		return Down
	case pitch <= -45:
		return Up
	}

	return HorizontalFacing(yaw)
}

// PlaceContext describes a click that places a block.
type PlaceContext struct {
	// Face is the face of the clicked block.
	Face Direction

	// CursorY is where the face was clicked, from 0 at its bottom to 1 at its top.
	CursorY float32

	// Yaw and Pitch are the player's rotation.
	Yaw, Pitch float32

	// Replacing is the state at the position the block goes to.
	Replacing StateID
}

// Placement is a state to place, offset from the target position.
type Placement struct {
	DX, DY, DZ int
	State      StateID
}

// waterSource is set once the block table is loaded.
var waterSource StateID

// Place returns the states a click with b places, starting with the target
// position. Orientation follows the vanilla rules for the common block shapes;
// two block objects such as doors and beds return both parts.
func (b *Block) Place(ctx PlaceContext) []Placement {
	target := b
	props := make(map[string]string)

	// Torches, signs, banners, heads and coral fans have a separate block
	// for being attached to the side of another block.
	if ctx.Face.horizontal() {
		if wall, ok := b.wallVariant(); ok {
			target = wall
			props["facing"] = ctx.Face.Name()
		}
	}

	facing := HorizontalFacing(ctx.Yaw)
	top := ctx.Face == Down || (ctx.Face.horizontal() && ctx.CursorY > 0.5)

	for _, p := range target.Properties {
		switch {
		case p.Name == "axis" && slices.Contains(p.Values, ctx.Face.axis()):
			props["axis"] = ctx.Face.axis()
		case p.Name == "face":
			switch {
			case ctx.Face == Up:
				props["face"], props["facing"] = "floor", facing.Name()
			case ctx.Face == Down:
				props["face"], props["facing"] = "ceiling", facing.Name()
			default:
				props["face"], props["facing"] = "wall", ctx.Face.Name()
			}
		case p.Name == "facing" && props["facing"] == "":
			props["facing"] = target.facingFor(ctx, facing, p.Values).Name()
		case p.Name == "half" && slices.Contains(p.Values, "top"):
			props["half"] = pick(top, "top", "bottom")
		case p.Name == "half":
			props["half"] = "lower"
		case p.Name == "type" && slices.Contains(p.Values, "double"):
			props["type"] = pick(top, "top", "bottom")
		case p.Name == "part":
			props["part"] = "foot"
		case p.Name == "rotation":
			props["rotation"] = strconv.Itoa(int(math.Floor(float64(ctx.Yaw+180)*16/360+0.5)) & 15)
		case p.Name == "waterlogged":
			props["waterlogged"] = pick(ctx.Replacing == waterSource, "true", "false")
		}
	}

	state, err := target.State(props)

	if err != nil {
		state = target.DefaultState
	}

	out := []Placement{{State: state}}

	if dx, dy, dz, ok := LinkedPart(state); ok {
		values := target.StateValues(state)

		switch values["half"] {
		case "lower":
			values["half"] = "upper"
		default:
			values["part"] = "head"
		}

		if other, err := target.State(values); err == nil {
			out = append(out, Placement{DX: dx, DY: dy, DZ: dz, State: other})
		}
	}

	return out
}

// facingFor picks the facing of a newly placed block.
func (b *Block) facingFor(ctx PlaceContext, player Direction, values []string) Direction {
	name := b.Name

	switch {
	case len(values) == 6 && (strings.HasSuffix(name, "_rod") || strings.Contains(name, "amethyst")):
		return ctx.Face
	case len(values) == 6:
		return lookDirection(ctx.Yaw, ctx.Pitch).Opposite()
	case strings.HasSuffix(name, "_trapdoor") && ctx.Face.horizontal():
		return ctx.Face
	case strings.HasSuffix(name, "_stairs"), strings.HasSuffix(name, "_door"), strings.HasSuffix(name, "_fence_gate"),
		strings.HasSuffix(name, "_bed"):
		return player
	}

	// Most directional blocks, like furnaces and chests, face the player.
	return player.Opposite()
}

// wallVariant returns the block placed when b is attached to a wall, such as
// wall_torch for torch or oak_wall_sign for oak_sign.
func (b *Block) wallVariant() (*Block, bool) {
	if hasProperty(b, "facing") {
		return nil, false
	}

	name := strings.TrimPrefix(b.Name, "minecraft:")
	candidates := []string{"wall_" + name}

	for i, c := range name {
		if c == '_' {
			candidates = append(candidates, name[:i]+"_wall"+name[i:])
		}
	}

	for _, c := range candidates {
		if wall, ok := ByName(c); ok && hasProperty(wall, "facing") {
			return wall, true
		}
	}

	return nil, false
}

// LinkedPart returns the offset to the other block of a two block object,
// such as the other half of a door, a tall flower or a bed.
func LinkedPart(s StateID) (dx, dy, dz int, ok bool) {
	b := ByState(s)
	values := b.StateValues(s)

	switch values["half"] {
	case "lower":
		return 0, 1, 0, true
	case "upper":
		return 0, -1, 0, true
	}

	facing, hasFacing := values["facing"]

	if !hasFacing {
		return 0, 0, 0, false
	}

	switch values["part"] {
	case "foot":
		dx, dy, dz = directionNamed(facing).Offset()
		return dx, dy, dz, true
	case "head":
		dx, dy, dz = directionNamed(facing).Opposite().Offset()
		return dx, dy, dz, true
	}

	return 0, 0, 0, false
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}

	return b
}
