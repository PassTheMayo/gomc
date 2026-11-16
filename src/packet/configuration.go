package packet

import (
	"maps"
	"slices"

	"main/src/nbt"
	"main/src/protocol"
	"main/src/registry"
)

// FinishConfiguration ends the configuration state. The server sends it once
// everything is configured and the client echoes it to enter play.
type FinishConfiguration struct{}

func (p *FinishConfiguration) Encode(*protocol.Writer) {}
func (p *FinishConfiguration) Decode(*protocol.Reader) {}

// KnownPack identifies a data pack both sides may already have.
type KnownPack struct {
	Namespace string
	ID        string
	Version   string
}

// SelectKnownPacks lists the data packs a side knows. The server offers its
// packs and the client answers with those it has, letting registry entries be
// sent without their contents.
type SelectKnownPacks struct {
	Packs []KnownPack
}

func (p *SelectKnownPacks) Encode(w *protocol.Writer) {
	protocol.WriteList(w, p.Packs, func(k KnownPack) {
		w.String(k.Namespace)
		w.String(k.ID)
		w.String(k.Version)
	})
}

func (p *SelectKnownPacks) Decode(r *protocol.Reader) {
	p.Packs = protocol.ReadList(r, 64, func() KnownPack {
		return KnownPack{Namespace: r.String(32767), ID: r.String(32767), Version: r.String(32767)}
	})
}

// RegistryEntry is an entry of a synchronized registry. Data is nil when the
// client already knows the entry from a known pack.
type RegistryEntry struct {
	ID   string
	Data nbt.Tag
}

// RegistryData sends the entries of one synchronized registry.
type RegistryData struct {
	Registry string
	Entries  []RegistryEntry
}

func (p *RegistryData) Encode(w *protocol.Writer) {
	w.String(p.Registry)
	protocol.WriteList(w, p.Entries, func(e RegistryEntry) {
		w.String(e.ID)
		w.Bool(e.Data != nil)

		if e.Data != nil {
			writeNBT(w, e.Data)
		}
	})
}

func (p *RegistryData) Decode(r *protocol.Reader) {
	p.Registry = r.String(32767)
	p.Entries = protocol.ReadList(r, r.Remaining(), func() RegistryEntry {
		e := RegistryEntry{ID: r.String(32767)}

		if r.Bool() {
			e.Data = readNBT(r)
		}

		return e
	})
}

// UpdateEnabledFeatures lists the enabled feature flags.
type UpdateEnabledFeatures struct {
	Features []string
}

func (p *UpdateEnabledFeatures) Encode(w *protocol.Writer) {
	protocol.WriteList(w, p.Features, w.String)
}

func (p *UpdateEnabledFeatures) Decode(r *protocol.Reader) {
	p.Features = protocol.ReadList(r, r.Remaining(), func() string { return r.String(32767) })
}

// UpdateTags sends registry tags, keyed by registry name.
type UpdateTags struct {
	Tags map[string][]registry.Tag
}

func (p *UpdateTags) Encode(w *protocol.Writer) {
	names := slices.Sorted(maps.Keys(p.Tags))

	protocol.WriteList(w, names, func(name string) {
		w.String(name)
		protocol.WriteList(w, p.Tags[name], func(t registry.Tag) {
			w.String(t.Name)
			protocol.WriteList(w, t.Entries, w.VarInt)
		})
	})
}

func (p *UpdateTags) Decode(r *protocol.Reader) {
	p.Tags = make(map[string][]registry.Tag)
	n := r.Length(r.Remaining())

	for i := 0; i < n && r.Err() == nil; i++ {
		name := r.String(32767)
		p.Tags[name] = protocol.ReadList(r, r.Remaining(), func() registry.Tag {
			return registry.Tag{Name: r.String(32767), Entries: protocol.ReadList(r, r.Remaining(), r.VarInt)}
		})
	}
}
