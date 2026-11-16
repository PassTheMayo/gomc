package server

import (
	"math"

	"main/src/packet"
	"main/src/protocol"
)

const (
	// positionSyncInterval is how often, in ticks, a moving player's absolute
	// position is resent to correct rounding drift in relative moves.
	positionSyncInterval = 60

	// maxRelativeMove is the largest delta, in 1/4096 blocks, a relative move
	// packet can carry.
	maxRelativeMove = math.MaxInt16
)

// trackedState is what viewers were last told about a player.
type trackedState struct {
	x, y, z    int64
	yaw, pitch byte
	headYaw    byte
	onGround   bool
	flags      int8
	pose       int32
	skinParts  uint8
	sinceSync  int
}

// encodePosition converts a coordinate to the 1/4096 block units of relative moves.
func encodePosition(v float64) int64 {
	return int64(math.Round(v * 4096))
}

func newTrackedState(snap snapshot) trackedState {
	return trackedState{
		x:         encodePosition(snap.X),
		y:         encodePosition(snap.Y),
		z:         encodePosition(snap.Z),
		yaw:       protocol.AngleToByte(snap.Yaw),
		pitch:     protocol.AngleToByte(snap.Pitch),
		headYaw:   protocol.AngleToByte(snap.Yaw),
		onGround:  snap.OnGround,
		flags:     entityFlags(snap),
		pose:      entityPose(snap),
		skinParts: snap.SkinParts,
	}
}

func entityFlags(snap snapshot) int8 {
	var flags int8

	if snap.Sneaking && !snap.Flying {
		flags |= packet.FlagCrouching
	}

	if snap.Sprinting {
		flags |= packet.FlagSprinting
	}

	return flags
}

func entityPose(snap snapshot) int32 {
	if snap.Sneaking && !snap.Flying {
		return packet.PoseCrouching
	}

	return packet.PoseStanding
}

// update compares a player's current state with what viewers last saw and
// returns the packets that bring them up to date.
func (t *trackedState) update(id int32, snap snapshot) []packet.Packet {
	var out []packet.Packet

	x, y, z := encodePosition(snap.X), encodePosition(snap.Y), encodePosition(snap.Z)
	dx, dy, dz := x-t.x, y-t.y, z-t.z
	yaw, pitch := protocol.AngleToByte(snap.Yaw), protocol.AngleToByte(snap.Pitch)
	moved := dx != 0 || dy != 0 || dz != 0
	rotated := yaw != t.yaw || pitch != t.pitch
	tooFar := abs64(dx) > maxRelativeMove || abs64(dy) > maxRelativeMove || abs64(dz) > maxRelativeMove

	t.sinceSync++

	switch {
	case moved && (tooFar || t.sinceSync >= positionSyncInterval):
		out = append(out, &packet.EntityPositionSync{
			EntityID: id, X: snap.X, Y: snap.Y, Z: snap.Z, Yaw: snap.Yaw, Pitch: snap.Pitch, OnGround: snap.OnGround,
		})
		t.sinceSync = 0
	case moved && rotated:
		out = append(out, &packet.MoveEntityPosRot{
			EntityID: id, DX: int16(dx), DY: int16(dy), DZ: int16(dz), Yaw: snap.Yaw, Pitch: snap.Pitch, OnGround: snap.OnGround,
		})
	case moved:
		out = append(out, &packet.MoveEntityPos{EntityID: id, DX: int16(dx), DY: int16(dy), DZ: int16(dz), OnGround: snap.OnGround})
	case rotated || snap.OnGround != t.onGround:
		out = append(out, &packet.MoveEntityRot{EntityID: id, Yaw: snap.Yaw, Pitch: snap.Pitch, OnGround: snap.OnGround})
	}

	if yaw != t.headYaw {
		out = append(out, &packet.RotateHead{EntityID: id, HeadYaw: snap.Yaw})
		t.headYaw = yaw
	}

	var data []packet.EntityDataValue

	if flags := entityFlags(snap); flags != t.flags {
		data = append(data, packet.EntityDataValue{Index: packet.DataSharedFlags, Type: packet.DataTypeByte, Byte: flags})
		t.flags = flags
	}

	if pose := entityPose(snap); pose != t.pose {
		data = append(data, packet.EntityDataValue{Index: packet.DataPose, Type: packet.DataTypePose, Pose: pose})
		t.pose = pose
	}

	if snap.SkinParts != t.skinParts {
		data = append(data, packet.EntityDataValue{Index: packet.DataPlayerModeCustomisation, Type: packet.DataTypeByte, Byte: int8(snap.SkinParts)})
		t.skinParts = snap.SkinParts
	}

	if data != nil {
		out = append(out, &packet.SetEntityData{EntityID: id, Values: data})
	}

	t.x, t.y, t.z = x, y, z
	t.yaw, t.pitch = yaw, pitch
	t.onGround = snap.OnGround

	return out
}

// spawnPackets returns the packets that make a player visible to a viewer.
func spawnPackets(p *Player, snap snapshot) []packet.Packet {
	return []packet.Packet{
		&packet.AddEntity{
			EntityID: p.EntityID,
			UUID:     p.Profile.ID,
			Type:     playerEntityType,
			X:        snap.X,
			Y:        snap.Y,
			Z:        snap.Z,
			Pitch:    snap.Pitch,
			Yaw:      snap.Yaw,
			HeadYaw:  snap.Yaw,
		},
		&packet.SetEntityData{EntityID: p.EntityID, Values: []packet.EntityDataValue{
			{Index: packet.DataSharedFlags, Type: packet.DataTypeByte, Byte: entityFlags(snap)},
			{Index: packet.DataPose, Type: packet.DataTypePose, Pose: entityPose(snap)},
			{Index: packet.DataPlayerModeCustomisation, Type: packet.DataTypeByte, Byte: int8(snap.SkinParts)},
		}},
	}
}

// trackEntities shows and hides players as they move in and out of each
// other's view distance, and relays their movement.
func (s *Server) trackEntities(players []*Player) {
	snaps := make(map[*Player]snapshot, len(players))
	updates := make(map[*Player][]packet.Packet, len(players))
	online := make(map[int32]bool, len(players))

	for _, p := range players {
		snap := p.snapshot()
		snaps[p] = snap
		updates[p] = p.broadcast.update(p.EntityID, snap)
		online[p.EntityID] = true
	}

	for _, viewer := range players {
		vs := snaps[viewer]
		var removed []int32

		for id, target := range viewer.visible {
			if !online[id] || !canSee(vs, snaps[target]) {
				removed = append(removed, id)
				delete(viewer.visible, id)
			}
		}

		if removed != nil {
			viewer.send(&packet.RemoveEntities{EntityIDs: removed})
		}

		for _, target := range players {
			if target == viewer {
				continue
			}

			if _, ok := viewer.visible[target.EntityID]; ok {
				for _, pk := range updates[target] {
					viewer.send(pk)
				}
			} else if canSee(vs, snaps[target]) {
				for _, pk := range spawnPackets(target, snaps[target]) {
					viewer.send(pk)
				}

				viewer.visible[target.EntityID] = target
			}
		}
	}
}

// canSee reports whether the target stands in a chunk the viewer has in view.
func canSee(viewer, target snapshot) bool {
	return inView(viewer.chunkPos(), viewer.ViewDistance, target.chunkPos())
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}

	return v
}
