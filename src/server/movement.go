package server

import (
	"math"

	"main/src/packet"
	"main/src/text"
	"main/src/world"
)

// maxCoordinate bounds accepted positions, like the vanilla world border.
const maxCoordinate = 3.0e7

var errInvalidMove = &kickError{reason: text.Translatable("multiplayer.disconnect.invalid_player_movement")}

// registerMovement handles player movement, teleports and the movement state
// other players see, and places joining players at their position.
func (s *Server) registerMovement() {
	r := &s.router

	r.onJoin(func(p *Player) {
		snap := p.snapshot()
		p.teleport(snap.X, snap.Y, snap.Z, snap.Yaw, snap.Pitch)

		p.Send(&packet.SetDefaultSpawnPosition{Spawn: packet.GlobalPos{
			Dimension: s.world.Dim.Name,
			X:         int32(s.spawn.X),
			Y:         int32(s.spawn.Y),
			Z:         int32(s.spawn.Z),
		}})
	})

	on(r, func(p *Player, pk *packet.AcceptTeleportation) { p.acceptTeleport(pk.TeleportID) })

	onChecked(r, func(p *Player, pk *packet.MovePlayerPos) error {
		return p.move(&[3]float64{pk.X, pk.Y, pk.Z}, nil, pk.OnGround)
	})

	onChecked(r, func(p *Player, pk *packet.MovePlayerPosRot) error {
		return p.move(&[3]float64{pk.X, pk.Y, pk.Z}, &[2]float32{pk.Yaw, pk.Pitch}, pk.OnGround)
	})

	onChecked(r, func(p *Player, pk *packet.MovePlayerRot) error {
		return p.move(nil, &[2]float32{pk.Yaw, pk.Pitch}, pk.OnGround)
	})

	onChecked(r, func(p *Player, pk *packet.MovePlayerStatusOnly) error {
		return p.move(nil, nil, pk.OnGround)
	})

	on(r, func(p *Player, pk *packet.PlayerInput) { p.setSneaking(pk.Flags&packet.InputSneak != 0) })
	on(r, func(p *Player, pk *packet.PlayerAbilities) { p.setFlying(pk.Flags&packet.AbilityFlying != 0) })

	on(r, func(p *Player, pk *packet.PlayerCommand) {
		switch pk.Action {
		case packet.CommandStartSprinting:
			p.setSprinting(true)
		case packet.CommandStopSprinting:
			p.setSprinting(false)
		}
	})
}

// teleport moves the player and ignores its movement until the client
// confirms the teleport.
func (p *Player) teleport(x, y, z float64, yaw, pitch float32) {
	p.mu.Lock()
	p.teleportID++
	id := p.teleportID
	p.awaitingTeleport = true
	p.x, p.y, p.z, p.yaw, p.pitch = x, y, z, yaw, pitch
	p.mu.Unlock()

	p.Send(&packet.PlayerPosition{TeleportID: id, X: x, Y: y, Z: z, Yaw: yaw, Pitch: pitch})
}

func (p *Player) acceptTeleport(id int32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if id == p.teleportID {
		p.awaitingTeleport = false
	}
}

// move applies a movement packet. A nil position or rotation means it did
// not change. The client is authoritative over its own movement; the server
// only rejects values that cannot be valid.
func (p *Player) move(pos *[3]float64, rot *[2]float32, onGround bool) error {
	if pos != nil {
		for i, v := range pos {
			limit := maxCoordinate

			if i == 1 {
				limit = 2.0e7
			}

			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > limit {
				return errInvalidMove
			}
		}
	}

	if rot != nil && (math.IsNaN(float64(rot[0])) || math.IsInf(float64(rot[0]), 0) || math.IsNaN(float64(rot[1])) || math.IsInf(float64(rot[1]), 0)) {
		return errInvalidMove
	}

	p.mu.Lock()

	if p.awaitingTeleport {
		p.mu.Unlock()
		return nil
	}

	before := world.ChunkPosAt(p.x, p.z)

	if pos != nil {
		p.x, p.y, p.z = pos[0], pos[1], pos[2]
	}

	if rot != nil {
		p.yaw, p.pitch = rot[0], max(-90, min(rot[1], 90))
	}

	p.onGround = onGround
	after := world.ChunkPosAt(p.x, p.z)
	p.mu.Unlock()

	if after != before {
		p.chunks.moveTo(after)
	}

	return nil
}

func (p *Player) setSneaking(v bool) {
	p.mu.Lock()
	p.sneaking = v
	p.mu.Unlock()
}

func (p *Player) setSprinting(v bool) {
	p.mu.Lock()
	p.sprinting = v
	p.mu.Unlock()
}

func (p *Player) setFlying(v bool) {
	p.mu.Lock()
	p.flying = v
	p.mu.Unlock()
}
