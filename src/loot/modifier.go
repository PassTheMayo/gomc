package loot

import (
	"encoding/json"
	"fmt"

	"main/src/block"
	"main/src/item"
	"main/src/protocol"
	"main/src/registry"
)

// modifier changes a generated stack, like setting its count.
type modifier interface {
	apply(s item.Stack, ctx *Context) item.Stack
}

var (
	modifierTypes = decoders[modifier]{}

	// blockStateComponent is the item component holding the block state
	// properties a block item places with.
	blockStateComponent = registry.MustGet(registry.DataComponentType).MustID("minecraft:block_state")
)

// decodeModifiers decodes a modifier or an array of them, each of which may
// have its own condition.
func decodeModifiers(raw json.RawMessage) ([]modifier, error) {
	return decodeList(raw, func(e json.RawMessage) (modifier, error) {
		m, err := modifierTypes.decode("modifier", e)

		if err != nil {
			return nil, err
		}

		var c struct {
			Condition json.RawMessage `json:"condition"`
		}

		if err := json.Unmarshal(e, &c); err != nil || c.Condition == nil {
			return m, err
		}

		cond, err := decodeCondition(c.Condition)

		return conditional{cond: cond, modifier: m}, err
	})
}

// applyAll applies modifiers in order.
func applyAll(mods []modifier, s item.Stack, ctx *Context) item.Stack {
	for _, m := range mods {
		s = m.apply(s, ctx)
	}

	return s
}

func init() {
	modifierTypes["minecraft:set_count"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type      string          `json:"type"`
			Condition json.RawMessage `json:"condition"`
			Count     json.RawMessage `json:"count"`
			Add       bool            `json:"add"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		count, err := decodeNumber(v.Count)

		return setCount{count: count, add: v.Add}, err
	}

	modifierTypes["minecraft:limit_count"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type      string          `json:"type"`
			Condition json.RawMessage `json:"condition"`
			Limit     levelRange      `json:"limit"`
		}

		return limitCount{v.Limit}, decodeStrict(raw, &v)
	}

	modifierTypes["minecraft:explosion_decay"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type      string          `json:"type"`
			Condition json.RawMessage `json:"condition"`
		}

		return explosionDecay{}, decodeStrict(raw, &v)
	}

	modifierTypes["minecraft:apply_bonus"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type        string          `json:"type"`
			Condition   json.RawMessage `json:"condition"`
			Enchantment string          `json:"enchantment"`
			Formula     string          `json:"formula"`
			Parameters  struct {
				BonusMultiplier int32   `json:"bonusMultiplier"`
				Extra           int32   `json:"extra"`
				Probability     float32 `json:"probability"`
			} `json:"parameters"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		b := applyBonus{enchantment: item.Enchantment(v.Enchantment)}
		p := v.Parameters

		switch v.Formula {
		case "minecraft:ore_drops":
			b.formula = oreDrops
		case "minecraft:uniform_bonus_count":
			b.formula = func(count, level int32, ctx *Context) int32 {
				return count + ctx.Rand.Int32N(p.BonusMultiplier*level+1)
			}
		case "minecraft:binomial_with_bonus_count":
			b.formula = func(count, level int32, ctx *Context) int32 {
				for range level + p.Extra {
					if ctx.Rand.Float32() < p.Probability {
						count++
					}
				}

				return count
			}
		default:
			return nil, fmt.Errorf("unsupported apply_bonus formula %q", v.Formula)
		}

		return b, nil
	}

	modifierTypes["minecraft:copy_state"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type       string          `json:"type"`
			Condition  json.RawMessage `json:"condition"`
			Block      string          `json:"block"`
			Properties []string        `json:"properties"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		b, ok := block.ByName(v.Block)

		if !ok {
			return nil, fmt.Errorf("copy_state of unknown block %s", v.Block)
		}

		return copyState{block: b, properties: v.Properties}, nil
	}

	// The server keeps no block entity data, so there is nothing to copy.
	modifierTypes["minecraft:copy_components"] = func(raw json.RawMessage) (modifier, error) {
		var v struct {
			Type      string          `json:"type"`
			Condition json.RawMessage `json:"condition"`
			Source    string          `json:"source"`
			Include   []string        `json:"include"`
		}

		if err := decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		if v.Source != "block_entity" {
			return nil, fmt.Errorf("copy_components from %q is not supported", v.Source)
		}

		return unchanged{}, nil
	}
}

// conditional applies a modifier only when its condition holds.
type conditional struct {
	cond     condition
	modifier modifier
}

func (c conditional) apply(s item.Stack, ctx *Context) item.Stack {
	if !c.cond.test(ctx) {
		return s
	}

	return c.modifier.apply(s, ctx)
}

type setCount struct {
	count number
	add   bool
}

func (c setCount) apply(s item.Stack, ctx *Context) item.Stack {
	n := c.count.int(ctx)

	if c.add {
		n += s.Count
	}

	return s.WithCount(n)
}

type limitCount struct {
	limit levelRange
}

func (l limitCount) apply(s item.Stack, _ *Context) item.Stack {
	if l.limit.Min != nil {
		s.Count = max(s.Count, *l.limit.Min)
	}

	if l.limit.Max != nil {
		s.Count = min(s.Count, *l.limit.Max)
	}

	return s
}

// explosionDecay keeps each item with a chance shrinking with the radius of
// the explosion that broke the block.
type explosionDecay struct{}

func (explosionDecay) apply(s item.Stack, ctx *Context) item.Stack {
	if ctx.ExplosionRadius <= 0 {
		return s
	}

	var kept int32

	for range s.Count {
		if ctx.Rand.Float32() <= 1/ctx.ExplosionRadius {
			kept++
		}
	}

	return s.WithCount(kept)
}

// applyBonus raises the count by the tool's level of an enchantment, such as
// fortune on ores.
type applyBonus struct {
	enchantment int32
	formula     func(count, level int32, ctx *Context) int32
}

func (b applyBonus) apply(s item.Stack, ctx *Context) item.Stack {
	return s.WithCount(b.formula(s.Count, ctx.enchantmentLevel(b.enchantment), ctx))
}

// oreDrops multiplies the count by a random factor from 1 to level+1, where
// a factor of 1 is twice as likely as the others.
func oreDrops(count, level int32, ctx *Context) int32 {
	if level <= 0 {
		return count
	}

	return count * (max(ctx.Rand.Int32N(level+2)-1, 0) + 1)
}

// copyState records some of the broken block's property values on the item,
// like a beehive's honey level.
type copyState struct {
	block      *block.Block
	properties []string
}

func (c copyState) apply(s item.Stack, ctx *Context) item.Stack {
	if block.ByState(ctx.State) != c.block {
		return s
	}

	values := c.block.StateValues(ctx.State)
	copied := make(map[string]string, len(c.properties))

	for _, name := range c.properties {
		if v, ok := values[name]; ok {
			copied[name] = v
		}
	}

	w := protocol.NewWriter()
	w.VarInt(int32(len(copied)))

	for name, v := range copied {
		w.String(name)
		w.String(v)
	}

	s.Components = append(append([]item.Component(nil), s.Components...), item.Component{Type: blockStateComponent, Data: w.Bytes()})

	return s
}

type unchanged struct{}

func (unchanged) apply(s item.Stack, _ *Context) item.Stack { return s }
