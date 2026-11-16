package loot

import (
	"bytes"
	"encoding/json"
	"fmt"

	"main/src/block"
	"main/src/item"
	"main/src/registry"
)

// condition decides whether a pool, entry or modifier applies.
type condition interface {
	test(ctx *Context) bool
}

var conditionTypes = decoders[condition]{}

// decodeCondition decodes a condition, where an array means all of them.
func decodeCondition(raw json.RawMessage) (condition, error) {
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		terms, err := decodeList(raw, decodeCondition)

		return allOf(terms), err
	}

	return conditionTypes.decode("condition", raw)
}

// passes reports whether an optional condition holds.
func passes(c condition, ctx *Context) bool {
	return c == nil || c.test(ctx)
}

func init() {
	conditionTypes["minecraft:all_of"] = func(raw json.RawMessage) (condition, error) {
		terms, err := decodeTerms(raw)

		return allOf(terms), err
	}

	conditionTypes["minecraft:any_of"] = func(raw json.RawMessage) (condition, error) {
		terms, err := decodeTerms(raw)

		return anyOf(terms), err
	}

	conditionTypes["minecraft:inverted"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type string          `json:"type"`
			Term json.RawMessage `json:"term"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		term, err := decodeCondition(v.Term)

		return inverted{term}, err
	}

	conditionTypes["minecraft:survives_explosion"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type string `json:"type"`
		}

		return survivesExplosion{}, decodeStrict(raw, &v)
	}

	conditionTypes["minecraft:random_chance"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type   string          `json:"type"`
			Chance json.RawMessage `json:"chance"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		chance, err := decodeNumber(v.Chance)

		return randomChance{chance}, err
	}

	conditionTypes["minecraft:table_bonus"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type        string    `json:"type"`
			Enchantment string    `json:"enchantment"`
			Chances     []float32 `json:"chances"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		if len(v.Chances) == 0 {
			return nil, fmt.Errorf("table_bonus without chances")
		}

		return tableBonus{enchantment: item.Enchantment(v.Enchantment), chances: v.Chances}, nil
	}

	conditionTypes["minecraft:match_block"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type string `json:"type"`
			blockPredicateJSON
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		pred, err := v.decode()

		return matchBlock{pred}, err
	}

	conditionTypes["minecraft:location_check"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type      string `json:"type"`
			OffsetX   int    `json:"offsetX"`
			OffsetY   int    `json:"offsetY"`
			OffsetZ   int    `json:"offsetZ"`
			Predicate struct {
				Block blockPredicateJSON `json:"block"`
			} `json:"predicate"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		pred, err := v.Predicate.Block.decode()

		return locationCheck{dx: v.OffsetX, dy: v.OffsetY, dz: v.OffsetZ, block: pred}, err
	}

	conditionTypes["minecraft:entity_properties"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type      string   `json:"type"`
			Entity    string   `json:"entity"`
			Predicate struct{} `json:"predicate"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		if v.Entity != "this" {
			return nil, fmt.Errorf("entity_properties of %q is not supported", v.Entity)
		}

		return entityPresent{}, nil
	}

	conditionTypes["minecraft:match_tool"] = func(raw json.RawMessage) (condition, error) {
		var v struct {
			Type      string `json:"type"`
			Predicate struct {
				Items      json.RawMessage `json:"items"`
				Predicates struct {
					Enchantments []struct {
						Enchantments json.RawMessage `json:"enchantments"`
						Levels       levelRange      `json:"levels"`
					} `json:"minecraft:enchantments"`
				} `json:"predicates"`
			} `json:"predicate"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		var m matchTool
		var err error

		if v.Predicate.Items != nil {
			if m.items, err = decodeSet(v.Predicate.Items, registry.Item); err != nil {
				return nil, err
			}
		}

		for _, e := range v.Predicate.Predicates.Enchantments {
			set, err := decodeSet(e.Enchantments, registry.Enchantment)

			if err != nil {
				return nil, err
			}

			m.enchantments = append(m.enchantments, enchantmentPredicate{enchantments: set, levels: e.Levels})
		}

		return m, nil
	}
}

func decodeTerms(raw json.RawMessage) ([]condition, error) {
	var v struct {
		Type  string          `json:"type"`
		Terms json.RawMessage `json:"terms"`
	}

	if err := decodeStrict(raw, &v); err != nil {
		return nil, err
	}

	return decodeList(v.Terms, decodeCondition)
}

type allOf []condition

func (a allOf) test(ctx *Context) bool {
	for _, c := range a {
		if !c.test(ctx) {
			return false
		}
	}

	return true
}

type anyOf []condition

func (a anyOf) test(ctx *Context) bool {
	for _, c := range a {
		if c.test(ctx) {
			return true
		}
	}

	return false
}

type inverted struct {
	term condition
}

func (i inverted) test(ctx *Context) bool { return !i.term.test(ctx) }

// survivesExplosion passes unless an explosion destroys the drop, which it
// does with a chance growing with the explosion's radius.
type survivesExplosion struct{}

func (survivesExplosion) test(ctx *Context) bool {
	return ctx.ExplosionRadius <= 0 || ctx.Rand.Float32() <= 1/ctx.ExplosionRadius
}

type randomChance struct {
	chance number
}

func (r randomChance) test(ctx *Context) bool { return ctx.Rand.Float32() < r.chance.float(ctx) }

// tableBonus passes with a chance picked by the tool's level of an
// enchantment, such as fortune for saplings from leaves.
type tableBonus struct {
	enchantment int32
	chances     []float32
}

func (t tableBonus) test(ctx *Context) bool {
	level := min(int(ctx.enchantmentLevel(t.enchantment)), len(t.chances)-1)

	return ctx.Rand.Float32() < t.chances[level]
}

// matchBlock passes for blocks of a type with given property values, like
// fully grown wheat.
type matchBlock struct {
	block blockPredicate
}

func (m matchBlock) test(ctx *Context) bool { return m.block.matches(ctx.State) }

// locationCheck passes when the block at an offset matches, like the other
// half of tall grass.
type locationCheck struct {
	dx, dy, dz int
	block      blockPredicate
}

func (l locationCheck) test(ctx *Context) bool {
	return ctx.BlockAt != nil && l.block.matches(ctx.BlockAt(l.dx, l.dy, l.dz))
}

// entityPresent passes when an entity broke the block.
type entityPresent struct{}

func (entityPresent) test(ctx *Context) bool { return ctx.ByEntity }

// matchTool passes when the tool is one of a set of items and has every
// listed enchantment.
type matchTool struct {
	items        map[int32]bool
	enchantments []enchantmentPredicate
}

func (m matchTool) test(ctx *Context) bool {
	if ctx.Tool.IsEmpty() || m.items != nil && !m.items[ctx.Tool.Item] {
		return false
	}

	for _, e := range m.enchantments {
		if !e.matches(ctx.Tool) {
			return false
		}
	}

	return true
}

// enchantmentPredicate matches stacks with one of a set of enchantments at a
// level in range.
type enchantmentPredicate struct {
	enchantments map[int32]bool
	levels       levelRange
}

func (e enchantmentPredicate) matches(s item.Stack) bool {
	for id := range e.enchantments {
		if level := s.EnchantmentLevel(id); level > 0 && e.levels.contains(level) {
			return true
		}
	}

	return false
}

// levelRange is an inclusive range whose missing bounds are unbounded.
type levelRange struct {
	Min *int32 `json:"min"`
	Max *int32 `json:"max"`
}

func (r levelRange) contains(v int32) bool {
	return (r.Min == nil || v >= *r.Min) && (r.Max == nil || v <= *r.Max)
}

// blockPredicateJSON is a block type and the property values it must have.
type blockPredicateJSON struct {
	Blocks json.RawMessage   `json:"blocks"`
	State  map[string]string `json:"state"`
}

func (b blockPredicateJSON) decode() (blockPredicate, error) {
	blocks, err := decodeSet(b.Blocks, registry.Block)

	return blockPredicate{blocks: blocks, state: b.State}, err
}

type blockPredicate struct {
	blocks map[int32]bool
	state  map[string]string
}

func (b blockPredicate) matches(s block.StateID) bool {
	owner := block.ByState(s)

	if owner == nil || !b.blocks[owner.ID] {
		return false
	}

	values := owner.StateValues(s)

	for name, want := range b.state {
		if values[name] != want {
			return false
		}
	}

	return true
}
