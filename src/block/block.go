// Package block provides the block state palette of the supported Minecraft
// version. States are numbered exactly like the vanilla global palette.
package block

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// blocks.json lists every block in registry order as [name, properties, default
// state offset]. Properties are sorted by name with values in protocol order.
//
//go:embed blocks.json
var blocksJSON []byte

// StateID is an index into the global block state palette.
type StateID int32

// Property is a block state property and its possible values.
type Property struct {
	Name   string
	Values []string
}

// Block is a block type and the range of states it occupies.
type Block struct {
	ID           int32
	Name         string
	Properties   []Property
	FirstState   StateID
	DefaultState StateID
	StateCount   int
}

var (
	blocks     []*Block
	byName     = make(map[string]*Block)
	stateOwner []uint16
)

func init() {
	var raw []json.RawMessage

	if err := json.Unmarshal(blocksJSON, &raw); err != nil {
		panic(fmt.Errorf("block: decoding blocks.json: %w", err))
	}

	next := StateID(0)

	for i, entry := range raw {
		var name string
		var props []json.RawMessage
		var defaultOffset int

		if err := decodeTuple(entry, &name, &props, &defaultOffset); err != nil {
			panic(fmt.Errorf("block: decoding block %d: %w", i, err))
		}

		b := &Block{ID: int32(i), Name: "minecraft:" + name, FirstState: next, StateCount: 1}

		for _, p := range props {
			var prop Property

			if err := decodeTuple(p, &prop.Name, &prop.Values); err != nil {
				panic(fmt.Errorf("block: decoding properties of %s: %w", b.Name, err))
			}

			b.Properties = append(b.Properties, prop)
			b.StateCount *= len(prop.Values)
		}

		b.DefaultState = next + StateID(defaultOffset)
		next += StateID(b.StateCount)

		blocks = append(blocks, b)
		byName[b.Name] = b

		for range b.StateCount {
			stateOwner = append(stateOwner, uint16(i))
		}
	}

	initBehaviors()
	initItemsAndEntities()
	waterSource = MustParse("water[level=0]")
}

// decodeTuple decodes a JSON array element by element into targets.
func decodeTuple(data json.RawMessage, targets ...any) error {
	var elems []json.RawMessage

	if err := json.Unmarshal(data, &elems); err != nil {
		return err
	}

	if len(elems) != len(targets) {
		return fmt.Errorf("expected %d elements, got %d", len(targets), len(elems))
	}

	for i, target := range targets {
		if err := json.Unmarshal(elems[i], target); err != nil {
			return err
		}
	}

	return nil
}

// StateCount returns the size of the global block state palette.
func StateCount() int {
	return len(stateOwner)
}

// Names returns every block name in registry order.
func Names() []string {
	names := make([]string, len(blocks))

	for i, b := range blocks {
		names[i] = b.Name
	}

	return names
}

// ByName looks up a block by its namespaced or bare name.
func ByName(name string) (*Block, bool) {
	if !strings.Contains(name, ":") {
		name = "minecraft:" + name
	}

	b, ok := byName[name]

	return b, ok
}

// ByState returns the block owning state s, or nil if s is out of range.
func ByState(s StateID) *Block {
	if s < 0 || int(s) >= len(stateOwner) {
		return nil
	}

	return blocks[stateOwner[s]]
}

// State returns the state with the given property values. Properties that are
// not specified keep their value from the default state.
func (b *Block) State(props map[string]string) (StateID, error) {
	defaults := b.StateValues(b.DefaultState)
	index, stride := 0, 1

	for _, p := range slices.Backward(b.Properties) {
		value, ok := props[p.Name]

		if !ok {
			value = defaults[p.Name]
		}

		v := slices.Index(p.Values, value)

		if v < 0 {
			return 0, fmt.Errorf("block: %s has no value %q for property %q", b.Name, value, p.Name)
		}

		index += v * stride
		stride *= len(p.Values)
	}

	for name := range props {
		if !slices.ContainsFunc(b.Properties, func(p Property) bool { return p.Name == name }) {
			return 0, fmt.Errorf("block: %s has no property %q", b.Name, name)
		}
	}

	return b.FirstState + StateID(index), nil
}

// StateValues returns the property values of state s, which must belong to b.
func (b *Block) StateValues(s StateID) map[string]string {
	values := make(map[string]string, len(b.Properties))
	index := int(s - b.FirstState)

	for _, p := range slices.Backward(b.Properties) {
		values[p.Name] = p.Values[index%len(p.Values)]
		index /= len(p.Values)
	}

	return values
}

// String formats s like "minecraft:oak_log[axis=y]".
func (s StateID) String() string {
	b := ByState(s)

	if b == nil {
		return fmt.Sprintf("invalid_state(%d)", s)
	}

	if len(b.Properties) == 0 {
		return b.Name
	}

	values := b.StateValues(s)
	parts := make([]string, len(b.Properties))

	for i, p := range b.Properties {
		parts[i] = p.Name + "=" + values[p.Name]
	}

	return b.Name + "[" + strings.Join(parts, ",") + "]"
}

// Parse parses a state written like "oak_log[axis=y]" or "minecraft:stone".
func Parse(spec string) (StateID, error) {
	name, rest, hasProps := strings.Cut(spec, "[")
	b, ok := ByName(name)

	if !ok {
		return 0, fmt.Errorf("block: unknown block %q", name)
	}

	props := make(map[string]string)

	if hasProps {
		rest, ok = strings.CutSuffix(rest, "]")

		if !ok {
			return 0, fmt.Errorf("block: malformed state %q", spec)
		}

		for pair := range strings.SplitSeq(rest, ",") {
			k, v, ok := strings.Cut(pair, "=")

			if !ok {
				return 0, fmt.Errorf("block: malformed state %q", spec)
			}

			props[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	return b.State(props)
}

// MustParse is like Parse but panics on error. It is meant for states that are
// known at compile time.
func MustParse(spec string) StateID {
	s, err := Parse(spec)

	if err != nil {
		panic(err)
	}

	return s
}
