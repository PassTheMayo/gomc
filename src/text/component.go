// Package text implements Minecraft's rich text components.
package text

import (
	"encoding/json"

	"main/src/nbt"
)

// Named colors understood by the client.
const (
	Red    = "red"
	Yellow = "yellow"
	Green  = "green"
	Gray   = "gray"
)

// Component is a piece of formatted chat text, either literal or translatable.
type Component struct {
	Text      string
	Translate string
	With      []Component
	Color     string
	Bold      bool
	Italic    bool
	Extra     []Component
}

// Text returns a literal text component.
func Text(s string) Component {
	return Component{Text: s}
}

// Translatable returns a component resolved by the client's language file.
func Translatable(key string, args ...Component) Component {
	return Component{Translate: key, With: args}
}

// Colored returns a copy of c with the given color.
func (c Component) Colored(color string) Component {
	c.Color = color

	return c
}

// Append returns a copy of c with children appended to its extra list.
func (c Component) Append(children ...Component) Component {
	c.Extra = append(append([]Component(nil), c.Extra...), children...)

	return c
}

// fields returns the component as an ordered list of key/value pairs shared by
// the JSON and NBT encodings.
func (c Component) fields() []field {
	var out []field

	if c.Translate != "" {
		out = append(out, field{"translate", c.Translate})

		if len(c.With) > 0 {
			out = append(out, field{"with", c.With})
		}
	} else {
		out = append(out, field{"text", c.Text})
	}

	if c.Color != "" {
		out = append(out, field{"color", c.Color})
	}

	if c.Bold {
		out = append(out, field{"bold", true})
	}

	if c.Italic {
		out = append(out, field{"italic", true})
	}

	if len(c.Extra) > 0 {
		out = append(out, field{"extra", c.Extra})
	}

	return out
}

type field struct {
	key   string
	value any
}

// MarshalJSON encodes c in the JSON form used by the status and login protocols.
func (c Component) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)

	for _, f := range c.fields() {
		m[f.key] = f.value
	}

	return json.Marshal(m)
}

// NBT encodes c in the NBT form used by the configuration and play protocols.
func (c Component) NBT() nbt.Compound {
	var out nbt.Compound

	for _, f := range c.fields() {
		switch v := f.value.(type) {
		case string:
			out.Set(f.key, nbt.String(v))
		case bool:
			out.Set(f.key, nbt.Byte(1))
		case []Component:
			items := make([]nbt.Tag, len(v))

			for i, child := range v {
				items[i] = child.NBT()
			}

			out.Set(f.key, nbt.NewList(items...))
		}
	}

	return out
}

// PlainText returns the literal text of c and its children, ignoring formatting.
func (c Component) PlainText() string {
	s := c.Text

	if c.Translate != "" {
		s = c.Translate
	}

	for _, child := range c.Extra {
		s += child.PlainText()
	}

	return s
}
