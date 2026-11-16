// Package registry provides the registries the server shares with clients:
// the data-driven registries synchronized during configuration and the
// built-in registries their tags refer to.
package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"main/src/block"
)

// synchronized.json lists the entries of every registry the client expects
// during configuration, in the order the vanilla server sends them. Entries
// are sorted, which also fixes their network IDs.
//
//go:embed synchronized.json
var synchronizedJSON []byte

// builtin.json lists built-in registries in protocol ID order.
//
//go:embed builtin.json
var builtinJSON []byte

// Registry names used by the server.
const (
	Biome         = "minecraft:worldgen/biome"
	DimensionType = "minecraft:dimension_type"
	WorldClock    = "minecraft:world_clock"
	Block         = "minecraft:block"
	EntityType    = "minecraft:entity_type"
	Item          = "minecraft:item"

	Enchantment       = "minecraft:enchantment"
	BlockEntityType   = "minecraft:block_entity_type"
	DataComponentType = "minecraft:data_component_type"
)

// Registry is an ordered list of entries whose index is their network ID.
type Registry struct {
	Name    string   `json:"name"`
	Entries []string `json:"entries"`
	ids     map[string]int32
}

var (
	synchronized []*Registry
	byName       = make(map[string]*Registry)
)

func init() {
	if err := json.Unmarshal(synchronizedJSON, &synchronized); err != nil {
		panic(fmt.Errorf("registry: decoding synchronized.json: %w", err))
	}

	var builtin map[string][]string

	if err := json.Unmarshal(builtinJSON, &builtin); err != nil {
		panic(fmt.Errorf("registry: decoding builtin.json: %w", err))
	}

	for _, r := range synchronized {
		register(r)
	}

	for name, entries := range builtin {
		register(&Registry{Name: name, Entries: entries})
	}

	register(&Registry{Name: Block, Entries: block.Names()})
}

func register(r *Registry) {
	r.ids = make(map[string]int32, len(r.Entries))

	for i, e := range r.Entries {
		r.ids[e] = int32(i)
	}

	byName[r.Name] = r
}

// Synchronized returns the registries sent to clients during configuration.
func Synchronized() []*Registry {
	return synchronized
}

// MustGet returns the registry with the given name and panics if it does not exist.
func MustGet(name string) *Registry {
	r, ok := byName[name]

	if !ok {
		panic("registry: unknown registry " + name)
	}

	return r
}

// ID returns the network ID of entry.
func (r *Registry) ID(entry string) (int32, bool) {
	id, ok := r.ids[entry]

	return id, ok
}

// MustID is like ID but panics if the entry does not exist.
func (r *Registry) MustID(entry string) int32 {
	id, ok := r.ids[entry]

	if !ok {
		panic(fmt.Sprintf("registry: %s has no entry %s", r.Name, entry))
	}

	return id
}
