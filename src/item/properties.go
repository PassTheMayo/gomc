package item

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"main/src/protocol"
	"main/src/registry"
)

// properties.json holds the default components of every item that isn't a
// plain stack of 64, from vanilla's item components report: its stack size,
// the armor or off-hand slot it is worn in and the tool rules that decide
// which blocks drop their loot when it breaks them.
//
//go:embed properties.json
var propertiesJSON []byte

// defaultMaxStackSize is the stack size of items without a max_stack_size
// default.
const defaultMaxStackSize = 64

// EquipmentSlot is the slot an item is worn in.
type EquipmentSlot uint8

// Equipment slots players wear items in.
const (
	SlotNone EquipmentSlot = iota
	SlotFeet
	SlotLegs
	SlotChest
	SlotHead
	SlotOffHand
)

var equipmentSlots = map[string]EquipmentSlot{
	"feet":    SlotFeet,
	"legs":    SlotLegs,
	"chest":   SlotChest,
	"head":    SlotHead,
	"offhand": SlotOffHand,
}

// ToolRule says whether a tool is the correct one for harvesting a set of
// blocks.
type ToolRule struct {
	Blocks          map[int32]bool
	CorrectForDrops bool
}

// Properties are an item's default components that the server acts on.
type Properties struct {
	MaxStackSize int32
	Equippable   EquipmentSlot
	Tool         []ToolRule
}

// CorrectForDrops reports whether the item harvests a block, by its block
// registry ID: the first tool rule listing the block decides, and items
// without one harvest nothing that needs a tool.
func (p Properties) CorrectForDrops(block int32) bool {
	for _, r := range p.Tool {
		if r.Blocks[block] {
			return r.CorrectForDrops
		}
	}

	return false
}

var (
	itemRegistry = registry.MustGet(registry.Item)

	// properties holds every item's properties, indexed by item ID.
	properties = loadProperties()

	maxStackSize        = registry.MustGet(registry.DataComponentType).MustID("minecraft:max_stack_size")
	enchantmentRegistry = registry.MustGet(registry.Enchantment)
)

func loadProperties() []Properties {
	var raw map[string]struct {
		MaxStackSize int32  `json:"max_stack_size"`
		Equippable   string `json:"equippable"`
		Tool         []struct {
			Blocks          string `json:"blocks"`
			CorrectForDrops bool   `json:"correct_for_drops"`
		} `json:"tool"`
	}

	if err := json.Unmarshal(propertiesJSON, &raw); err != nil {
		panic(fmt.Errorf("item: decoding properties.json: %w", err))
	}

	out := make([]Properties, len(itemRegistry.Entries))

	for i := range out {
		out[i].MaxStackSize = defaultMaxStackSize
	}

	for name, r := range raw {
		p := &out[itemRegistry.MustID("minecraft:"+name)]
		p.Equippable = equipmentSlots[r.Equippable]

		if r.MaxStackSize > 0 {
			p.MaxStackSize = r.MaxStackSize
		}

		for _, rule := range r.Tool {
			p.Tool = append(p.Tool, ToolRule{Blocks: blockSet(rule.Blocks), CorrectForDrops: rule.CorrectForDrops})
		}
	}

	return out
}

// blockSet resolves a block name or #tag to block registry IDs.
func blockSet(name string) map[int32]bool {
	set := make(map[int32]bool)

	tag, isTag := strings.CutPrefix(name, "#")

	if !isTag {
		set[registry.MustGet(registry.Block).MustID(name)] = true
		return set
	}

	entries, ok := registry.TagEntries(registry.Block, tag)

	if !ok {
		panic(fmt.Errorf("item: unknown block tag %s", tag))
	}

	for _, id := range entries {
		set[id] = true
	}

	return set
}

// PropertiesOf returns an item's properties. Unknown IDs get those of a plain
// item.
func PropertiesOf(item int32) Properties {
	if item < 0 || int(item) >= len(properties) {
		return Properties{MaxStackSize: defaultMaxStackSize}
	}

	return properties[item]
}

// MaxStackSize returns how many items fit in the stack, honouring a changed
// max_stack_size component.
func (s Stack) MaxStackSize() int32 {
	if data, ok := s.component(maxStackSize); ok {
		return protocol.NewReader(data).VarInt()
	}

	return PropertiesOf(s.Item).MaxStackSize
}

// IsStackable reports whether more than one of the item fits in a stack.
func (s Stack) IsStackable() bool {
	return s.MaxStackSize() > 1
}

// EnchantmentLevel returns the level of an enchantment on the stack, by its
// enchantment registry ID, or 0.
func (s Stack) EnchantmentLevel(enchantment int32) int32 {
	data, ok := s.component(enchantments)

	if !ok {
		return 0
	}

	r := protocol.NewReader(data)

	for range r.Length(len(data)) {
		id, level := r.VarInt(), r.VarInt()

		if r.Err() == nil && id == enchantment {
			return level
		}
	}

	return 0
}

// Enchantment returns the registry ID of a vanilla enchantment, such as
// minecraft:fortune.
func Enchantment(name string) int32 {
	return enchantmentRegistry.MustID(name)
}

// component returns the value of a component the stack adds or changes.
func (s Stack) component(t int32) ([]byte, bool) {
	for _, c := range s.Components {
		if c.Type == t {
			return c.Data, true
		}
	}

	return nil, false
}

// WithCount returns the stack with a different count.
func (s Stack) WithCount(count int32) Stack {
	s.Count = count

	return s
}
