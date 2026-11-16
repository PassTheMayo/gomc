package world

// Dimension describes the vertical layout of a level.
type Dimension struct {
	Name     string
	Type     string
	MinY     int
	Height   int
	SeaLevel int
}

// Overworld matches the vanilla overworld dimension type.
var Overworld = Dimension{
	Name:     "minecraft:overworld",
	Type:     "minecraft:overworld",
	MinY:     -64,
	Height:   384,
	SeaLevel: 63,
}

// Sections returns the number of 16 block tall sections in a chunk column.
func (d Dimension) Sections() int {
	return d.Height / 16
}

// MaxY returns the highest buildable Y coordinate.
func (d Dimension) MaxY() int {
	return d.MinY + d.Height - 1
}
