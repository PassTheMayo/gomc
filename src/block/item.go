package block

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
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

// pickedItems maps blocks without an item of their own to the item vanilla
// gives for them in getCloneItemStack, or in the Items class when the item
// also places the block. Potted plants, candle cakes and wall-mounted
// variants follow rules instead.
var pickedItems = map[string]string{
	"water_cauldron":        "cauldron",
	"lava_cauldron":         "cauldron",
	"powder_snow_cauldron":  "cauldron",
	"big_dripleaf_stem":     "big_dripleaf",
	"tall_seagrass":         "seagrass",
	"attached_pumpkin_stem": "pumpkin_seeds",
	"attached_melon_stem":   "melon_seeds",
	"kelp_plant":            "kelp",
	"weeping_vines_plant":   "weeping_vines",
	"twisting_vines_plant":  "twisting_vines",
	"cave_vines_plant":      "glow_berries",
	"bamboo_sapling":        "bamboo",
	"piston_head":           "piston",
}

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

	for _, b := range blocks {
		if blockItems[b.ID] == "" {
			blockItems[b.ID] = pickedItem(b)
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

// pickedItem returns the item picked from a block that is not placed by an
// item, or "" if it has none.
func pickedItem(b *Block) string {
	name := strings.TrimPrefix(b.Name, "minecraft:")

	switch {
	case pickedItems[name] != "":
		return "minecraft:" + pickedItems[name]
	case strings.HasSuffix(name, "candle_cake"):
		return "minecraft:cake"
	case strings.HasPrefix(name, "potted_"):
		plant := strings.TrimPrefix(name, "potted_")

		// potted_azalea_bush holds an azalea.
		if _, ok := byName["minecraft:"+plant]; !ok {
			plant = strings.TrimSuffix(plant, "_bush")
		}

		if p, ok := byName["minecraft:"+plant]; ok {
			return blockItems[p.ID]
		}
	}

	return ""
}

// PickItem returns the item a player gets by middle-clicking state s, if any.
func PickItem(s StateID) (string, bool) {
	b := ByState(s)

	if b == nil || blockItems[b.ID] == "" {
		return "", false
	}

	if b.Name == "minecraft:piston_head" && b.StateValues(s)["type"] == "sticky" {
		return "minecraft:sticky_piston", true
	}

	return blockItems[b.ID], true
}

// EntityType returns the block entity type of state s, if it has one.
func EntityType(s StateID) (string, bool) {
	b := ByState(s)

	if b == nil || !hasEntity[b.ID] {
		return "", false
	}

	return entityTypes[b.ID], true
}
