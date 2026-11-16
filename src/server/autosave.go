package server

import "errors"

// autosaveTicks is how often changed chunks, players and the level are saved,
// like the vanilla server's five minute autosave.
const autosaveTicks = 6000

// registerAutosave saves the world in the background every five minutes.
func (s *Server) registerAutosave() {
	s.ticks.everyAsync(autosaveTicks, &s.wg, func(t *tick) { s.autosave(t.players) })
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
