package server

import (
	"main/src/packet"
	"main/src/registry"
)

// timeSyncTicks is how often the game time is sent to clients.
const timeSyncTicks = 20

var overworldClock = registry.MustGet(registry.WorldClock).MustID("minecraft:overworld")

// registerGameTime advances the game time every tick and keeps clients' day
// cycle in sync.
func (s *Server) registerGameTime() {
	s.router.onJoin(func(p *Player) { p.Send(s.timePacket()) })
	s.ticks.every(1, func(*tick) { s.gameTime.Add(1) })

	s.ticks.every(timeSyncTicks, func(t *tick) {
		update := s.timePacket()

		for _, p := range t.players {
			p.Send(update)
		}
	})
}

// timePacket returns the current game time and overworld clock.
func (s *Server) timePacket() *packet.SetTime {
	t := s.gameTime.Load()

	return &packet.SetTime{GameTime: t, Clocks: []packet.ClockState{{Clock: overworldClock, TotalTicks: t, Rate: 1}}}
}
