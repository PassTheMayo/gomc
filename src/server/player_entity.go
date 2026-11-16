package server

import (
	"main/src/entity"
	"main/src/packet"
	"main/src/registry"
	"main/src/uuid"
	"main/src/world"
)

var playerEntityType = registry.MustGet(registry.EntityType).MustID("minecraft:player")

// ID implements entity.Entity and entity.Viewer.
func (p *Player) ID() int32 {
	return p.EntityID
}

// UUID implements entity.Entity.
func (p *Player) UUID() uuid.UUID {
	return p.Profile.ID
}

// Type implements entity.Entity.
func (p *Player) Type() int32 {
	return playerEntityType
}

// State implements entity.Entity: the player's position, rotation, sneaking
// and sprinting, and skin layers.
func (p *Player) State() entity.State {
	snap := p.snapshot()
	crouching := snap.Sneaking && !snap.Flying

	var flags int8
	pose := packet.PoseStanding

	if crouching {
		flags |= packet.FlagCrouching
		pose = packet.PoseCrouching
	}

	if snap.Sprinting {
		flags |= packet.FlagSprinting
	}

	return entity.State{
		X: snap.X, Y: snap.Y, Z: snap.Z,
		Yaw: snap.Yaw, Pitch: snap.Pitch, HeadYaw: snap.Yaw,
		OnGround: snap.OnGround,
		Metadata: []packet.EntityDataValue{
			{Index: packet.DataSharedFlags, Type: packet.DataTypeByte, Byte: flags},
			{Index: packet.DataPose, Type: packet.DataTypePose, Pose: pose},
			{Index: packet.DataPlayerModeCustomisation, Type: packet.DataTypeByte, Byte: int8(snap.SkinParts)},
		},
	}
}

// View implements entity.Viewer.
func (p *Player) View() (world.ChunkPos, int) {
	snap := p.snapshot()

	return snap.chunkPos(), snap.ViewDistance
}

// registerEntityTracking shows players to each other as they move in and out
// of view and relays their movement every tick.
func (s *Server) registerEntityTracking() {
	s.ticks.every(1, func(t *tick) {
		entities := make([]entity.Entity, len(t.players))
		viewers := make([]entity.Viewer, len(t.players))

		for i, p := range t.players {
			entities[i], viewers[i] = p, p
		}

		s.entities.Update(entities, viewers)
	})
}
