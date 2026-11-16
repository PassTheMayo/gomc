package loot

import (
	"encoding/json"

	"main/src/item"
	"main/src/registry"
)

// table is a loot table: pools rolled independently, whose items all pass
// through the table's modifiers.
type table struct {
	pools     []pool
	modifiers []modifier
}

// pool rolls its entries a number of times, picking one entry per roll from
// those whose conditions pass.
type pool struct {
	rolls     number
	cond      condition
	entries   []entry
	modifiers []modifier
}

// entry offers generators for a pool to pick from. expand reports whether
// the entry applied, which ends an alternatives entry.
type entry interface {
	expand(ctx *Context, offer func(g generator)) bool
}

// generator makes the items of a picked entry.
type generator interface {
	generate(ctx *Context) []item.Stack
}

var entryTypes = decoders[entry]{}

// UnmarshalJSON decodes a table in vanilla's loot table format.
func (t *table) UnmarshalJSON(raw []byte) error {
	var v struct {
		Type     string            `json:"type"`
		Pools    []json.RawMessage `json:"pools"`
		Modifier json.RawMessage   `json:"modifier"`
	}

	if err := decodeStrict(raw, &v); err != nil {
		return err
	}

	mods, err := decodeModifiers(v.Modifier)

	if err != nil {
		return err
	}

	t.modifiers = mods

	for _, p := range v.Pools {
		decoded, err := decodePool(p)

		if err != nil {
			return err
		}

		t.pools = append(t.pools, decoded)
	}

	return nil
}

func decodePool(raw json.RawMessage) (pool, error) {
	var v struct {
		Rolls     json.RawMessage `json:"rolls"`
		Condition json.RawMessage `json:"condition"`
		Entries   json.RawMessage `json:"entries"`
		Modifier  json.RawMessage `json:"modifier"`
	}

	var p pool
	var err error

	if err = decodeStrict(raw, &v); err != nil {
		return p, err
	}

	if p.rolls, err = decodeNumber(v.Rolls); err != nil {
		return p, err
	}

	if v.Condition != nil {
		if p.cond, err = decodeCondition(v.Condition); err != nil {
			return p, err
		}
	}

	if p.entries, err = decodeList(v.Entries, decodeEntry); err != nil {
		return p, err
	}

	p.modifiers, err = decodeModifiers(v.Modifier)

	return p, err
}

func decodeEntry(raw json.RawMessage) (entry, error) {
	return entryTypes.decode("entry", raw)
}

// generate rolls the table and returns the items, split into stacks no
// larger than their item allows.
func (t *table) generate(ctx *Context) []item.Stack {
	var out []item.Stack

	for _, p := range t.pools {
		for _, s := range p.generate(ctx) {
			s = applyAll(t.modifiers, s, ctx)

			for s.Count > 0 {
				n := min(s.Count, s.MaxStackSize())
				out = append(out, s.WithCount(n))
				s.Count -= n
			}
		}
	}

	return out
}

func (p *pool) generate(ctx *Context) []item.Stack {
	if !passes(p.cond, ctx) {
		return nil
	}

	var out []item.Stack

	for range p.rolls.int(ctx) {
		var offered []generator

		for _, e := range p.entries {
			e.expand(ctx, func(g generator) { offered = append(offered, g) })
		}

		if len(offered) == 0 {
			continue
		}

		for _, s := range offered[ctx.Rand.IntN(len(offered))].generate(ctx) {
			out = append(out, applyAll(p.modifiers, s, ctx))
		}
	}

	return out
}

func init() {
	entryTypes["minecraft:item"] = func(raw json.RawMessage) (entry, error) {
		var v struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Condition json.RawMessage `json:"condition"`
			Modifier  json.RawMessage `json:"modifier"`
		}

		var e itemEntry
		var err error

		if err = decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		e.item = registry.MustGet(registry.Item).MustID(v.Name)

		if v.Condition != nil {
			if e.cond, err = decodeCondition(v.Condition); err != nil {
				return nil, err
			}
		}

		e.modifiers, err = decodeModifiers(v.Modifier)

		return e, err
	}

	entryTypes["minecraft:alternatives"] = func(raw json.RawMessage) (entry, error) {
		var v struct {
			Type      string          `json:"type"`
			Children  json.RawMessage `json:"children"`
			Condition json.RawMessage `json:"condition"`
		}

		var e alternatives
		var err error

		if err = decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		if v.Condition != nil {
			if e.cond, err = decodeCondition(v.Condition); err != nil {
				return nil, err
			}
		}

		e.children, err = decodeList(v.Children, decodeEntry)

		return e, err
	}

	// Dynamic entries drop block entity contents, like the sherds of a
	// cracked decorated pot, which the server doesn't keep.
	entryTypes["minecraft:dynamic"] = func(raw json.RawMessage) (entry, error) {
		var v struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Condition json.RawMessage `json:"condition"`
		}

		var e dynamic
		var err error

		if err = decodeStrict(raw, &v); err != nil {
			return nil, err
		}

		if v.Condition != nil {
			e.cond, err = decodeCondition(v.Condition)
		}

		return e, err
	}
}

// itemEntry generates one item, changed by its modifiers.
type itemEntry struct {
	item      int32
	cond      condition
	modifiers []modifier
}

func (e itemEntry) expand(ctx *Context, offer func(generator)) bool {
	if !passes(e.cond, ctx) {
		return false
	}

	offer(e)

	return true
}

func (e itemEntry) generate(ctx *Context) []item.Stack {
	return []item.Stack{applyAll(e.modifiers, item.Of(e.item, 1), ctx)}
}

// alternatives applies the first child that applies.
type alternatives struct {
	cond     condition
	children []entry
}

func (a alternatives) expand(ctx *Context, offer func(generator)) bool {
	if !passes(a.cond, ctx) {
		return false
	}

	for _, c := range a.children {
		if c.expand(ctx, offer) {
			return true
		}
	}

	return false
}

type dynamic struct {
	cond condition
}

func (d dynamic) expand(ctx *Context, offer func(generator)) bool {
	if !passes(d.cond, ctx) {
		return false
	}

	offer(d)

	return true
}

func (dynamic) generate(*Context) []item.Stack { return nil }
