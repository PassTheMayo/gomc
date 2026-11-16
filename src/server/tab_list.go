package server

import "main/src/packet"

// latencySyncTicks is how often tab list latencies are refreshed.
const latencySyncTicks = 100

// tabListAdd adds a player to the tab list with its game mode and latency.
const tabListAdd = packet.InfoAddPlayer | packet.InfoUpdateGameMode | packet.InfoUpdateListed | packet.InfoUpdateLatency

// registerTabList keeps the latencies in the tab list up to date. Players are
// added and removed as they join and leave.
func (s *Server) registerTabList() {
	s.ticks.every(latencySyncTicks, func(t *tick) {
		if len(t.players) == 0 {
			return
		}

		entries := make([]packet.PlayerInfoEntry, len(t.players))

		for i, p := range t.players {
			entries[i] = p.infoEntry(p.snapshot())
		}

		update := &packet.PlayerInfoUpdate{Actions: packet.InfoUpdateLatency, Entries: entries}

		for _, p := range t.players {
			p.Send(update)
		}
	})
}

func (p *Player) infoEntry(snap snapshot) packet.PlayerInfoEntry {
	return packet.PlayerInfoEntry{Profile: p.Profile, GameMode: p.GameMode(), Listed: true, Latency: snap.Latency}
}
