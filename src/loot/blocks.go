package loot

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"main/src/block"
	"main/src/item"
)

// blocks.json holds vanilla's block loot tables by block name, with named
// predicates inlined. Blocks that use another block's table, like wall
// torches, name that block instead.
//
//go:embed blocks.json
var blocksJSON []byte

// blockTables returns every block's loot table, indexed by block ID. Blocks
// without one, like air, have nil.
var blockTables = sync.OnceValue(func() []*table {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(blocksJSON, &raw); err != nil {
		panic(fmt.Errorf("loot: decoding blocks.json: %w", err))
	}

	byName := make(map[string]*table, len(raw))
	var aliases [][2]string

	for name, data := range raw {
		var alias string

		if json.Unmarshal(data, &alias) == nil {
			aliases = append(aliases, [2]string{name, alias})
			continue
		}

		t := &table{}

		if err := json.Unmarshal(data, t); err != nil {
			panic(fmt.Errorf("loot: decoding the loot table of %s: %w", name, err))
		}

		byName[name] = t
	}

	for _, a := range aliases {
		byName[a[0]] = byName[a[1]]
	}

	tables := make([]*table, len(block.Names()))

	for i, name := range block.Names() {
		tables[i] = byName[strings.TrimPrefix(name, "minecraft:")]
	}

	return tables
})

// BlockDrops returns the items a block drops when broken, from its loot
// table.
func BlockDrops(ctx *Context) []item.Stack {
	t := blockTables()[block.ByState(ctx.State).ID]

	if t == nil {
		return nil
	}

	return t.generate(ctx)
}
