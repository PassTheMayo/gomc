package block

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// correct_tool.json lists the blocks that drop nothing unless broken with the
// correct tool, derived from vanilla's block properties.
//
//go:embed correct_tool.json
var correctToolJSON []byte

// needsCorrectTool is indexed by block ID.
var needsCorrectTool []bool

func initHarvest() {
	var names []string

	if err := json.Unmarshal(correctToolJSON, &names); err != nil {
		panic(fmt.Errorf("block: decoding correct_tool.json: %w", err))
	}

	needsCorrectTool = make([]bool, len(blocks))

	for _, name := range names {
		b, ok := ByName(name)

		if !ok {
			panic(fmt.Errorf("block: correct_tool.json lists unknown block %s", name))
		}

		needsCorrectTool[b.ID] = true
	}
}

// RequiresCorrectTool reports whether s only drops its loot when broken with
// a tool whose rules mark it correct, like stone with a pickaxe.
func RequiresCorrectTool(s StateID) bool {
	return needsCorrectTool[ByState(s).ID]
}
