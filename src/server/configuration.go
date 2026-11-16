package server

import (
	"slices"
	"sync"

	"main/src/packet"
	"main/src/protocol"
	"main/src/registry"
	"main/src/text"
	"main/src/version"
)

// corePack is the vanilla data pack. When the client confirms it has the same
// version, registry entries can be sent without their contents.
var corePack = packet.KnownPack{Namespace: "minecraft", ID: "core", Version: version.Name}

var (
	registryPackets     []packet.Packet
	registryPacketsOnce sync.Once
)

// configurationPackets returns the registry and tag packets, which are the
// same for every player.
func configurationPackets() []packet.Packet {
	registryPacketsOnce.Do(func() {
		for _, r := range registry.Synchronized() {
			entries := make([]packet.RegistryEntry, len(r.Entries))

			for i, e := range r.Entries {
				entries[i] = packet.RegistryEntry{ID: e}
			}

			registryPackets = append(registryPackets, &packet.RegistryData{Registry: r.Name, Entries: entries})
		}

		registryPackets = append(registryPackets, &packet.UpdateTags{Tags: registry.NetworkTags()})
	})

	return registryPackets
}

// defaultClientInformation is assumed until the client reports its settings.
var defaultClientInformation = packet.ClientInformation{Locale: "en_us", ViewDistance: 2, ChatColors: true, MainHand: 1}

// configure sends the server brand, feature flags, registries and tags. It
// returns false if the connection was rejected or lost.
func (s *session) configure() bool {
	s.clientInfo = defaultClientInformation

	brand := protocol.NewWriter()
	brand.String(Brand)

	s.send(&packet.CustomPayload{Channel: "minecraft:brand", Data: brand.Bytes()})

	if len(s.srv.links) > 0 {
		s.send(&packet.ServerLinks{Links: s.srv.links})
	}

	s.send(&packet.UpdateEnabledFeatures{Features: []string{"minecraft:vanilla"}})
	s.send(&packet.SelectKnownPacks{Packs: []packet.KnownPack{corePack}})

	for {
		p, err := s.read(true)

		if err != nil {
			return false
		}

		switch p := p.(type) {
		case *packet.ClientInformation:
			s.clientInfo = *p
		case *packet.SelectKnownPacks:
			if !slices.Contains(p.Packs, corePack) {
				s.disconnect(text.Text("This server requires the vanilla " + version.Name + " client"))
				return false
			}

			for _, rp := range configurationPackets() {
				s.send(rp)
			}

			s.send(&packet.FinishConfiguration{})
		case *packet.FinishConfiguration:
			s.setState(statePlay, packet.PlayServerbound, packet.PlayClientbound)
			return true
		}
	}
}
