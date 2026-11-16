package loot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"main/src/registry"
)

// decoders maps the JSON type of a loot table element to its decoder.
type decoders[T any] map[string]func(raw json.RawMessage) (T, error)

// decode decodes an element by the decoder registered for its "type".
func (d decoders[T]) decode(kind string, raw json.RawMessage) (T, error) {
	var head struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(raw, &head); err != nil {
		var zero T
		return zero, fmt.Errorf("decoding %s: %w", kind, err)
	}

	dec, ok := d[head.Type]

	if !ok {
		var zero T
		return zero, fmt.Errorf("unsupported %s type %q", kind, head.Type)
	}

	return dec(raw)
}

// decodeList decodes a single element or an array of them.
func decodeList[T any](raw json.RawMessage, decode func(json.RawMessage) (T, error)) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var elems []json.RawMessage

	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		if err := json.Unmarshal(raw, &elems); err != nil {
			return nil, err
		}
	} else {
		elems = []json.RawMessage{raw}
	}

	out := make([]T, len(elems))

	for i, e := range elems {
		v, err := decode(e)

		if err != nil {
			return nil, err
		}

		out[i] = v
	}

	return out, nil
}

// decodeStrict decodes an object into v, failing on fields v doesn't have so
// that unsupported options are noticed instead of ignored.
func decodeStrict(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	return dec.Decode(v)
}

// decodeSet resolves a registry entry name, #tag or list of names to network
// IDs in the named registry.
func decodeSet(raw json.RawMessage, registryName string) (map[int32]bool, error) {
	names, err := decodeList(raw, func(e json.RawMessage) (string, error) {
		var name string
		err := json.Unmarshal(e, &name)

		return name, err
	})

	if err != nil {
		return nil, err
	}

	set := make(map[int32]bool)

	for _, name := range names {
		if tag, isTag := strings.CutPrefix(name, "#"); isTag {
			entries, ok := registry.TagEntries(registryName, tag)

			if !ok {
				return nil, fmt.Errorf("unknown tag %s in %s", tag, registryName)
			}

			for _, id := range entries {
				set[id] = true
			}

			continue
		}

		id, ok := registry.MustGet(registryName).ID(name)

		if !ok {
			return nil, fmt.Errorf("unknown entry %s in %s", name, registryName)
		}

		set[id] = true
	}

	return set, nil
}
