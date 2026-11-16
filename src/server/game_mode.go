package server

import "main/src/packet"

// setGameMode switches the player to mode, updates the abilities it grants
// and shows the change in everyone's tab list. It reports false and does
// nothing if the player is already in mode.
func (s *Server) setGameMode(p *Player, mode int32) bool {
	if p.gameMode.Swap(mode) == mode {
		return false
	}

	// Like vanilla, creative players keep flying and spectators always fly.
	p.mu.Lock()

	switch mode {
	case packet.GameModeSurvival, packet.GameModeAdventure:
		p.flying = false
	case packet.GameModeSpectator:
		p.flying = true
	}

	flying := p.flying
	p.mu.Unlock()

	p.Send(&packet.GameEvent{Event: packet.GameEventChangeGameMode, Value: float32(mode)})
	p.Send(&packet.PlayerAbilitiesUpdate{
		Flags:        abilityFlags(mode, flying),
		FlyingSpeed:  packet.DefaultFlyingSpeed,
		WalkingSpeed: packet.DefaultWalkingSpeed,
	})

	s.broadcast(&packet.PlayerInfoUpdate{
		Actions: packet.InfoUpdateGameMode,
		Entries: []packet.PlayerInfoEntry{p.infoEntry(p.snapshot())},
	})

	return true
}

// abilityFlags returns the abilities a game mode grants a player that is or
// is not flying.
func abilityFlags(mode int32, flying bool) uint8 {
	var flags uint8

	switch mode {
	case packet.GameModeCreative:
		flags = packet.AbilityInvulnerable | packet.AbilityCanFly | packet.AbilityInstabuild
	case packet.GameModeSpectator:
		flags = packet.AbilityInvulnerable | packet.AbilityCanFly
	}

	if flying {
		flags |= packet.AbilityFlying
	}

	return flags
}
