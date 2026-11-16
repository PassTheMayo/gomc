package server

import (
	"errors"
	"sync/atomic"
	"time"

	"main/src/packet"
)

const (
	// tickDuration is the length of a game tick; the game runs at 20 ticks per second.
	tickDuration = 50 * time.Millisecond

	// timeSyncTicks is how often the game time is sent to clients.
	timeSyncTicks = 20

	// latencySyncTicks is how often tab list latencies are refreshed.
	latencySyncTicks = 100

	// unloadTicks is how often idle chunks are unloaded.
	unloadTicks = 600

	// chunkIdleTime is how long a chunk stays loaded without viewers.
	chunkIdleTime = 30 * time.Second

	// autosaveTicks is how often changed chunks, players and the level are
	// saved, like the vanilla server's five minute autosave.
	autosaveTicks = 6000
)

// tickLoop runs the game at a fixed rate.
func (s *Server) tickLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	var tick int64
	var unloading, saving atomic.Bool

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
		}

		tick++
		s.gameTime.Add(1)

		players := s.joinedPlayers()
		s.trackEntities(players)

		now := time.Now()

		for _, p := range players {
			p.tickKeepAlive(now)
		}

		if tick%timeSyncTicks == 0 {
			t := s.timePacket()

			for _, p := range players {
				p.send(t)
			}
		}

		if tick%latencySyncTicks == 0 && len(players) > 0 {
			entries := make([]packet.PlayerInfoEntry, len(players))

			for i, p := range players {
				entries[i] = p.infoEntry(p.snapshot())
			}

			update := &packet.PlayerInfoUpdate{Actions: packet.InfoUpdateLatency, Entries: entries}

			for _, p := range players {
				p.send(update)
			}
		}

		// Saving can block on disk, so it runs outside the tick. Shutdown
		// waits for it before closing the storage.
		if tick%autosaveTicks == 0 && saving.CompareAndSwap(false, true) {
			s.wg.Go(func() {
				defer saving.Store(false)
				s.autosave(players)
			})
		}

		if tick%unloadTicks == 0 && unloading.CompareAndSwap(false, true) {
			s.wg.Go(func() {
				defer unloading.Store(false)
				s.world.UnloadIdle(chunkIdleTime)
			})
		}
	}
}

// joinedPlayers returns the players that finished joining.
func (s *Server) joinedPlayers() []*Player {
	players := s.onlinePlayers()
	out := players[:0]

	for _, p := range players {
		if p.joined.Load() {
			out = append(out, p)
		}
	}

	return out
}

// playerCount returns the number of players in the world.
func (s *Server) playerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.players)
}

// autosave saves changed chunks, the given players and the level.
func (s *Server) autosave(players []*Player) {
	err := s.world.Save()

	for _, p := range players {
		err = errors.Join(err, s.store.SavePlayer(p.Profile.ID, p.savedData()))
	}

	level := s.level
	level.GameTime = s.gameTime.Load()

	if err = errors.Join(err, s.store.SaveLevel(level)); err != nil {
		s.log.Error("Autosave failed", "error", err)
	}
}
