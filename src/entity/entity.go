// Package entity tracks which entities each player can see and keeps them up
// to date: players now, and mobs, items and vehicles as they are added.
package entity

import (
	"main/src/packet"
	"main/src/uuid"
	"main/src/world"
)

// Entity is anything shown to players.
type Entity interface {
	// ID returns the entity's network ID.
	ID() int32

	// UUID returns the entity's persistent identity.
	UUID() uuid.UUID

	// Type returns the entity's entity_type registry ID.
	Type() int32

	// State returns what viewers can currently observe about the entity.
	State() State
}

// State is the observable state of an entity.
type State struct {
	X, Y, Z    float64
	Yaw, Pitch float32
	HeadYaw    float32
	OnGround   bool

	// VX, VY and VZ are the velocity clients start the entity with and
	// simulate it from, in blocks per tick.
	VX, VY, VZ float64

	// Impulse reports that the velocity changed in a way clients can't
	// predict, such as by bouncing, so it is sent again.
	Impulse bool

	// Metadata holds every synchronized data value, each index once.
	Metadata []packet.EntityDataValue
}

// ChunkPos returns the chunk the entity is in.
func (s State) ChunkPos() world.ChunkPos {
	return world.ChunkPosAt(s.X, s.Z)
}

// Viewer is a client that entities are shown to.
type Viewer interface {
	// ID returns the network ID of the viewer's own entity, which it never
	// tracks.
	ID() int32

	// View returns the chunk the viewer's view is centered on and its radius.
	View() (center world.ChunkPos, radius int)

	// Send sends a packet to the viewer.
	Send(pk packet.Packet)
}
