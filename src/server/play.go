package server

import (
	"errors"
	"time"

	"main/src/inventory"
	"main/src/packet"
	"main/src/registry"
	"main/src/text"
	"main/src/uuid"
	"main/src/world"
	"main/src/worldgen"
)

// play runs the play state until the connection ends.
func (s *session) play() {
	s.startQueue()

	p := s.srv.newPlayer(s)
	s.player = p

	s.srv.join(p)
	defer s.srv.leave(p)

	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(readTimeout))

		pk, err := s.read(true)

		if err != nil {
			return
		}

		if err = s.srv.router.dispatch(p, pk); err != nil {
			reason := text.Translatable("disconnect.packetError")

			if kick, ok := errors.AsType[*kickError](err); ok {
				reason = kick.reason
			}

			s.disconnect(reason)

			return
		}
	}
}

// newPlayer creates the player of a session at its saved position, or at the
// world spawn if it never played here before.
func (s *Server) newPlayer(sess *session) *Player {
	saved, found, err := s.store.LoadPlayer(sess.profile.ID)

	if err != nil {
		sess.log.Error("Failed to load player data, using the spawn point", "error", err)
	}

	if !found {
		saved = s.spawn
	}

	p := &Player{
		session:   sess,
		Profile:   sess.profile,
		EntityID:  s.nextEntityID.Add(1),
		operator:  s.isOperator(sess.profile.Name),
		x:         saved.X,
		y:         saved.Y,
		z:         saved.Z,
		yaw:       saved.Yaw,
		pitch:     saved.Pitch,
		inventory: inventory.NewPlayer(),
	}

	p.gameMode.Store(s.gameMode)
	p.inventory.Load(saved.HeldSlot, saved.Inventory)

	radius := max(2, min(int(sess.clientInfo.ViewDistance), s.conf.ViewDistance))
	p.viewDistance = radius
	p.skinParts = sess.clientInfo.ModelCustomisation
	p.chunks = newChunkView(p, s.world, world.ChunkPosAt(saved.X, saved.Z), radius)

	return p
}

// join puts the player into the world: it sends the login packet and every
// feature's join steps, adds the player to everyone's tab list and starts
// streaming chunks.
func (s *Server) join(p *Player) {
	s.mu.Lock()
	previous := s.players[p.Profile.ID]
	s.players[p.Profile.ID] = p
	s.mu.Unlock()

	if previous != nil {
		previous.session.disconnect(text.Translatable("multiplayer.disconnect.duplicate_login"))
	}

	snap := p.snapshot()

	p.Send(&packet.Login{
		EntityID:           p.EntityID,
		Dimensions:         []string{s.world.Dim.Name},
		MaxPlayers:         int32(s.conf.MaxPlayers),
		ViewDistance:       int32(s.conf.ViewDistance),
		SimulationDistance: int32(s.conf.SimulationDistance),
		ShowDeathScreen:    true,
		Spawn: packet.SpawnInfo{
			DimensionType:    registry.MustGet(registry.DimensionType).MustID(s.world.Dim.Type),
			Dimension:        s.world.Dim.Name,
			HashedSeed:       s.hashedSeed,
			GameMode:         p.GameMode(),
			PreviousGameMode: -1,
			Flat:             s.level.Generator == worldgen.NameFlat,
			SeaLevel:         int32(s.world.Dim.SeaLevel),
		},
		OnlineMode: s.conf.OnlineMode,
	})

	s.router.join(p)

	// The tab list update for everyone else is sent while holding the join
	// lock so concurrent joins agree on who is online.
	s.joinMu.Lock()
	entries := []packet.PlayerInfoEntry{p.infoEntry(snap)}

	for _, other := range s.onlinePlayers() {
		if other != p && other.joined.Load() {
			entries = append(entries, other.infoEntry(other.snapshot()))
			other.Send(&packet.PlayerInfoUpdate{Actions: tabListAdd, Entries: entries[:1]})
		}
	}

	p.Send(&packet.PlayerInfoUpdate{Actions: tabListAdd, Entries: entries})
	s.joinMu.Unlock()

	p.Send(&packet.GameEvent{Event: packet.GameEventLevelChunksLoadStart})
	p.chunks.start()
	p.joined.Store(true)

	s.broadcast(&packet.SystemChat{Content: text.Translatable("multiplayer.player.joined", text.Text(p.Profile.Name)).Colored(text.Yellow)})
	p.session.log.Info("Player joined", "uuid", p.Profile.ID.String(), "entity_id", p.EntityID)
}

// leave removes the player from the world and saves it.
func (s *Server) leave(p *Player) {
	p.chunks.stop()

	if err := s.store.SavePlayer(p.Profile.ID, p.savedData()); err != nil {
		p.session.log.Error("Failed to save player data", "error", err)
	}

	s.joinMu.Lock()
	defer s.joinMu.Unlock()

	s.mu.Lock()
	current := s.players[p.Profile.ID] == p

	if current {
		delete(s.players, p.Profile.ID)
	}

	s.mu.Unlock()

	// A player replaced by a newer login of the same account keeps its tab
	// list entry, which now belongs to the new session.
	if current {
		s.broadcast(&packet.PlayerInfoRemove{UUIDs: []uuid.UUID{p.Profile.ID}})
		s.broadcast(&packet.SystemChat{Content: text.Translatable("multiplayer.player.left", text.Text(p.Profile.Name)).Colored(text.Yellow)})
	}

	p.session.log.Info("Player left")
}
