package text

import (
	"encoding/json"

	"main/src/nbt"
)

// UnmarshalJSON decodes a component from a JSON string, object or array.
func (c *Component) UnmarshalJSON(data []byte) error {
	var s string

	if json.Unmarshal(data, &s) == nil {
		*c = Text(s)
		return nil
	}

	var list []Component

	if json.Unmarshal(data, &list) == nil {
		*c = Component{}

		if len(list) > 0 {
			*c = list[0]
			c.Extra = append(c.Extra, list[1:]...)
		}

		return nil
	}

	var obj struct {
		Text      string      `json:"text"`
		Translate string      `json:"translate"`
		With      []Component `json:"with"`
		Color     string      `json:"color"`
		Bold      bool        `json:"bold"`
		Italic    bool        `json:"italic"`
		Extra     []Component `json:"extra"`
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	*c = Component(obj)

	return nil
}

// FromNBT decodes a component from its NBT form.
func FromNBT(tag nbt.Tag) Component {
	switch t := tag.(type) {
	case nbt.String:
		return Text(string(t))
	case nbt.List:
		var c Component

		for i, item := range t.Items {
			if i == 0 {
				c = FromNBT(item)
			} else {
				c.Extra = append(c.Extra, FromNBT(item))
			}
		}

		return c
	case nbt.Compound:
		var c Component

		for _, e := range t {
			switch e.Name {
			case "text", "translate", "color":
				s, _ := e.Value.(nbt.String)

				switch e.Name {
				case "text":
					c.Text = string(s)
				case "translate":
					c.Translate = string(s)
				case "color":
					c.Color = string(s)
				}
			case "bold":
				b, _ := e.Value.(nbt.Byte)
				c.Bold = b != 0
			case "italic":
				b, _ := e.Value.(nbt.Byte)
				c.Italic = b != 0
			case "with", "extra":
				list, _ := e.Value.(nbt.List)
				children := make([]Component, len(list.Items))

				for i, item := range list.Items {
					children[i] = FromNBT(item)
				}

				if e.Name == "with" {
					c.With = children
				} else {
					c.Extra = children
				}
			}
		}

		return c
	}

	return Component{}
}
