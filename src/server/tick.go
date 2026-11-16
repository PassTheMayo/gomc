package server

import "time"

// tickDuration is the length of a game tick; the game runs at 20 ticks per
// second.
const tickDuration = 50 * time.Millisecond

// tickLoop runs the features' scheduled tasks at a fixed rate.
func (s *Server) tickLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(tickDuration)
	defer ticker.Stop()

	var number int64

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
		}

		number++
		s.ticks.run(&tick{number: number, now: time.Now(), players: s.joinedPlayers()})
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
