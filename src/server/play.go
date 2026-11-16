package server

import (
	"time"

	"main/src/packet"
	"main/src/registry"
	"main/src/text"
	"main/src/uuid"
	"main/src/world"
	"main/src/worldgen"
)

var (
	playerEntityType = registry.MustGet(registry.EntityType).MustID("minecraft:player")
	overworldClock   = registry.MustGet(registry.WorldClock).MustID("minecraft:overworld")
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

		if err = s.srv.handle(p, pk); err != nil {
			s.disconnect(text.Translatable("multiplayer.disconnect.invalid_player_movement"))
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
		session:  sess,
		Profile:  sess.profile,
		EntityID: s.nextEntityID.Add(1),
		GameMode: s.gameMode,
		operator: s.isOperator(sess.profile.Name),
		x:        saved.X,
		y:        saved.Y,
		z:        saved.Z,
		yaw:      saved.Yaw,
		pitch:    saved.Pitch,
		visible:  make(map[int32]*Player),
	}

	p.loadInventory(saved)

	radius := max(2, min(int(sess.clientInfo.ViewDistance), s.conf.ViewDistance))
	p.viewDistance = radius
	p.skinParts = sess.clientInfo.ModelCustomisation
	p.chunks = newChunkView(p, s.world, world.ChunkPosAt(saved.X, saved.Z), radius)

	return p
}

// join puts the player into the world: it sends the join sequence, adds the
// player to everyone's tab list and starts streaming chunks.
func (s *Server) join(p *Player) {
	s.mu.Lock()
	previous := s.players[p.Profile.ID]
	s.players[p.Profile.ID] = p
	s.mu.Unlock()

	if previous != nil {
		previous.session.disconnect(text.Translatable("multiplayer.disconnect.duplicate_login"))
	}

	snap := p.snapshot()

	p.send(&packet.Login{
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
			GameMode:         p.GameMode,
			PreviousGameMode: -1,
			Flat:             s.level.Generator == worldgen.NameFlat,
			SeaLevel:         int32(s.world.Dim.SeaLevel),
		},
		OnlineMode: s.conf.OnlineMode,
	})

	p.send(s.commands.Tree(p))

	for _, pk := range p.inventoryPackets() {
		p.send(pk)
	}

	p.teleport(snap.X, snap.Y, snap.Z, snap.Yaw, snap.Pitch)

	p.send(&packet.SetDefaultSpawnPosition{Spawn: packet.GlobalPos{
		Dimension: s.world.Dim.Name,
		X:         int32(s.spawn.X),
		Y:         int32(s.spawn.Y),
		Z:         int32(s.spawn.Z),
	}})

	// The tab list update for everyone else is sent while holding the join
	// lock so concurrent joins agree on who is online.
	s.joinMu.Lock()
	entries := []packet.PlayerInfoEntry{p.infoEntry(snap)}

	for _, other := range s.onlinePlayers() {
		if other != p && other.joined.Load() {
			entries = append(entries, other.infoEntry(other.snapshot()))
			other.send(&packet.PlayerInfoUpdate{Actions: tabListAdd, Entries: entries[:1]})
		}
	}

	p.send(&packet.PlayerInfoUpdate{Actions: tabListAdd, Entries: entries})
	p.broadcast = newTrackedState(snap)
	s.joinMu.Unlock()

	p.send(s.timePacket())
	p.send(&packet.GameEvent{Event: packet.GameEventLevelChunksLoadStart})
	p.chunks.start()
	p.joined.Store(true)

	s.broadcast(&packet.SystemChat{Content: text.Translatable("multiplayer.player.joined", text.Text(p.Profile.Name)).Colored(text.Yellow)})
	p.session.log.Info("Player joined", "uuid", p.Profile.ID.String(), "entity_id", p.EntityID)
}

// tabListAdd adds a player to the tab list with its game mode and latency.
const tabListAdd = packet.InfoAddPlayer | packet.InfoUpdateGameMode | packet.InfoUpdateListed | packet.InfoUpdateLatency

func (p *Player) infoEntry(snap snapshot) packet.PlayerInfoEntry {
	return packet.PlayerInfoEntry{Profile: p.Profile, GameMode: p.GameMode, Listed: true, Latency: snap.Latency}
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

// handle processes a play packet from p.
func (s *Server) handle(p *Player, pk packet.Packet) error {
	switch pk := pk.(type) {
	case *packet.AcceptTeleportation:
		p.acceptTeleport(pk.TeleportID)
	case *packet.MovePlayerPos:
		return p.move(&[3]float64{pk.X, pk.Y, pk.Z}, nil, pk.OnGround)
	case *packet.MovePlayerPosRot:
		return p.move(&[3]float64{pk.X, pk.Y, pk.Z}, &[2]float32{pk.Yaw, pk.Pitch}, pk.OnGround)
	case *packet.MovePlayerRot:
		return p.move(nil, &[2]float32{pk.Yaw, pk.Pitch}, pk.OnGround)
	case *packet.MovePlayerStatusOnly:
		return p.move(nil, nil, pk.OnGround)
	case *packet.KeepAlive:
		p.keepAliveResponse(pk.ID)
	case *packet.ChunkBatchReceived:
		p.chunks.acknowledge(pk.DesiredChunksPerTick)
	case *packet.ClientInformation:
		p.updateClientInformation(*pk, s.conf.ViewDistance)
	case *packet.PlayerInput:
		p.setSneaking(pk.Flags&packet.InputSneak != 0)
	case *packet.PlayerCommand:
		switch pk.Action {
		case packet.CommandStartSprinting:
			p.setSprinting(true)
		case packet.CommandStopSprinting:
			p.setSprinting(false)
		}
	case *packet.PlayerAbilities:
		p.setFlying(pk.Flags&packet.AbilityFlying != 0)
	case *packet.PlayerAction:
		switch pk.Action {
		case packet.ActionDropItem, packet.ActionDropAllItems:
			p.dropHeldItem(pk.Action == packet.ActionDropAllItems)
		default:
			s.handleDigging(p, pk)
		}
	case *packet.PickItemFromBlock:
		s.handlePickItemFromBlock(p, pk)
	case *packet.UseItemOn:
		s.handleUseItemOn(p, pk)
	case *packet.SetCarriedItem:
		p.setHeldSlot(pk.Slot)
	case *packet.SetCreativeModeSlot:
		p.setCreativeSlot(pk.Slot, pk.Stack)
	case *packet.ChatCommand:
		p.session.log.Info("Player ran a command", "command", pk.Command)
		s.commands.Execute(p, pk.Command)
	}

	return nil
}

// timePacket returns the current game time and overworld clock.
func (s *Server) timePacket() *packet.SetTime {
	t := s.gameTime.Load()

	return &packet.SetTime{GameTime: t, Clocks: []packet.ClockState{{Clock: overworldClock, TotalTicks: t, Rate: 1}}}
}
