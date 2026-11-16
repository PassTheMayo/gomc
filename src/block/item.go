package block

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// items.json maps every block item to the block it places. An empty value
// means the block has the same name as the item.
//
//go:embed items.json
var itemsJSON []byte

// entities.json maps every block with a block entity to the entity's type.
//
//go:embed entities.json
var entitiesJSON []byte

var (
	itemBlocks  = make(map[string]*Block)
	blockItems  []string
	entityTypes = make(map[int32]string)
	hasEntity   []bool
)

func initItemsAndEntities() {
	var items map[string]string

	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		panic(fmt.Errorf("block: decoding items.json: %w", err))
	}

	blockItems = make([]string, len(blocks))

	for item, name := range items {
		if name == "" {
			name = item
		}

		b := byName["minecraft:"+name]
		itemBlocks["minecraft:"+item] = b
		blockItems[b.ID] = "minecraft:" + item
	}

	// Wall-mounted variants share the item of the block they are placed as.
	for _, b := range blocks {
		if wall, ok := b.wallVariant(); ok && blockItems[b.ID] != "" && blockItems[wall.ID] == "" {
			blockItems[wall.ID] = blockItems[b.ID]
		}
	}

	var entities map[string]string

	if err := json.Unmarshal(entitiesJSON, &entities); err != nil {
		panic(fmt.Errorf("block: decoding entities.json: %w", err))
	}

	hasEntity = make([]bool, len(blocks))

	for name, typ := range entities {
		b := byName["minecraft:"+name]
		entityTypes[b.ID] = "minecraft:" + typ
		hasEntity[b.ID] = true
	}
}

// ForItem returns the block an item places, if it is a block item.
func ForItem(item string) (*Block, bool) {
	b, ok := itemBlocks[item]

	return b, ok
}

// PickItem returns the item a player gets by middle-clicking state s, if any.
func PickItem(s StateID) (string, bool) {
	b := ByState(s)

	if b == nil {
		return "", false
	}

	item := b.hooks.pickItem(b, s)

	return item, item != ""
}

// EntityType returns the block entity type of state s, if it has one.
func EntityType(s StateID) (string, bool) {
	b := ByState(s)

	if b == nil || !hasEntity[b.ID] {
		return "", false
	}

	return entityTypes[b.ID], true
}
