package registry

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
)

// tags.json holds every vanilla tag for the registries the server can number,
// with nested tag references already flattened.
//
//go:embed tags.json
var tagsJSON []byte

// Tag is a named set of registry entries, identified by network ID.
type Tag struct {
	Name    string
	Entries []int32
}

var (
	networkTags     map[string][]Tag
	networkTagsOnce sync.Once
)

// NetworkTags returns the tags of every registry, keyed by registry name, with
// entries resolved to network IDs.
func NetworkTags() map[string][]Tag {
	networkTagsOnce.Do(func() {
		var raw map[string]map[string][]string

		if err := json.Unmarshal(tagsJSON, &raw); err != nil {
			panic(fmt.Errorf("registry: decoding tags.json: %w", err))
		}

		networkTags = make(map[string][]Tag, len(raw))

		for regName, tags := range raw {
			r := MustGet(regName)
			out := make([]Tag, 0, len(tags))

			for name, entries := range tags {
				t := Tag{Name: name, Entries: make([]int32, len(entries))}

				for i, e := range entries {
					t.Entries[i] = r.MustID(e)
				}

				out = append(out, t)
			}

			slices.SortFunc(out, func(a, b Tag) int { return cmp.Compare(a.Name, b.Name) })

			networkTags[regName] = out
		}
	})

	return networkTags
}

// TagEntries returns the network IDs of a tag's entries, such as
// minecraft:mineable/pickaxe in the block registry.
func TagEntries(registryName, tag string) ([]int32, bool) {
	tags := NetworkTags()[registryName]
	i, found := slices.BinarySearchFunc(tags, tag, func(t Tag, name string) int { return cmp.Compare(t.Name, name) })

	if !found {
		return nil, false
	}

	return tags[i].Entries, true
}
