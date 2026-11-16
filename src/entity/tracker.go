package entity

import (
	"math"
	"sync"

	"main/src/packet"
	"main/src/protocol"
	"main/src/world"
)

const (
	// positionSyncInterval is how often, in updates, a moving entity's
	// absolute position is resent to correct rounding drift in relative moves.
	positionSyncInterval = 60

	// maxRelativeMove is the largest delta, in 1/4096 blocks, a relative move
	// packet can carry.
	maxRelativeMove = math.MaxInt16
)

// Tracker shows entities to the viewers whose view includes them, hides them
// when they leave it, and relays their movement and metadata. The game loop
// calls Update once per tick; SendToViewers may be called at any time.
type Tracker struct {
	mu      sync.Mutex
	sent    map[int32]*sentState
	viewers map[int32]Viewer
	visible map[int32]map[int32]bool
}

// NewTracker returns a tracker that shows nothing yet.
func NewTracker() *Tracker {
	return &Tracker{
		sent:    make(map[int32]*sentState),
		viewers: make(map[int32]Viewer),
		visible: make(map[int32]map[int32]bool),
	}
}

// SendToViewers sends a packet about an entity, such as an animation, to
// every viewer it is currently shown to.
func (t *Tracker) SendToViewers(id int32, pk packet.Packet) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for viewer, visible := range t.visible {
		if visible[id] {
			t.viewers[viewer].Send(pk)
		}
	}
}

// Update brings every viewer up to date with the given entities. Entities and
// viewers missing from a call are treated as gone.
func (t *Tracker) Update(entities []Entity, viewers []Viewer) {
	t.mu.Lock()
	defer t.mu.Unlock()

	states := make(map[int32]State, len(entities))
	updates := make(map[int32][]packet.Packet, len(entities))

	for _, e := range entities {
		id, state := e.ID(), e.State()
		states[id] = state

		// An entity's first state goes out in its spawn packets.
		if sent, ok := t.sent[id]; ok {
			updates[id] = sent.update(id, state)
		} else {
			t.sent[id] = newSentState(state)
		}
	}

	for id := range t.sent {
		if _, ok := states[id]; !ok {
			delete(t.sent, id)
		}
	}

	present := make(map[int32]bool, len(viewers))

	for _, v := range viewers {
		present[v.ID()] = true
		t.viewers[v.ID()] = v
		t.updateViewer(v, entities, states, updates)
	}

	for id := range t.viewers {
		if !present[id] {
			delete(t.viewers, id)
			delete(t.visible, id)
		}
	}
}

func (t *Tracker) updateViewer(v Viewer, entities []Entity, states map[int32]State, updates map[int32][]packet.Packet) {
	center, radius := v.View()
	visible := t.visible[v.ID()]

	if visible == nil {
		visible = make(map[int32]bool)
		t.visible[v.ID()] = visible
	}

	var removed []int32

	for id := range visible {
		if state, ok := states[id]; !ok || !world.InView(center, radius, state.ChunkPos()) {
			removed = append(removed, id)
			delete(visible, id)
		}
	}

	if removed != nil {
		v.Send(&packet.RemoveEntities{EntityIDs: removed})
	}

	for _, e := range entities {
		id := e.ID()

		switch {
		case id == v.ID():
		case visible[id]:
			for _, pk := range updates[id] {
				v.Send(pk)
			}
		case world.InView(center, radius, states[id].ChunkPos()):
			for _, pk := range spawnPackets(e, states[id]) {
				v.Send(pk)
			}

			visible[id] = true
		}
	}
}

// spawnPackets returns the packets that make an entity appear to a viewer.
func spawnPackets(e Entity, s State) []packet.Packet {
	out := []packet.Packet{&packet.AddEntity{
		EntityID: e.ID(), UUID: e.UUID(), Type: e.Type(),
		X: s.X, Y: s.Y, Z: s.Z, Pitch: s.Pitch, Yaw: s.Yaw, HeadYaw: s.HeadYaw,
	}}

	if len(s.Metadata) > 0 {
		out = append(out, &packet.SetEntityData{EntityID: e.ID(), Values: s.Metadata})
	}

	return out
}

// sentState is what viewers were last told about an entity, with positions in
// the 1/4096 block units of relative moves.
type sentState struct {
	x, y, z    int64
	yaw, pitch byte
	headYaw    byte
	onGround   bool
	metadata   map[uint8]packet.EntityDataValue
	sinceSync  int
}

func encodePosition(v float64) int64 {
	return int64(math.Round(v * 4096))
}

func newSentState(s State) *sentState {
	sent := &sentState{metadata: make(map[uint8]packet.EntityDataValue, len(s.Metadata))}
	sent.record(s)

	for _, v := range s.Metadata {
		sent.metadata[v.Index] = v
	}

	return sent
}

// record remembers a state's position and rotation as sent.
func (t *sentState) record(s State) {
	t.x, t.y, t.z = encodePosition(s.X), encodePosition(s.Y), encodePosition(s.Z)
	t.yaw, t.pitch = protocol.AngleToByte(s.Yaw), protocol.AngleToByte(s.Pitch)
	t.headYaw = protocol.AngleToByte(s.HeadYaw)
	t.onGround = s.OnGround
}

// update compares an entity's state with what viewers last saw and returns
// the packets that bring them up to date.
func (t *sentState) update(id int32, s State) []packet.Packet {
	var out []packet.Packet

	dx, dy, dz := encodePosition(s.X)-t.x, encodePosition(s.Y)-t.y, encodePosition(s.Z)-t.z
	yaw, pitch := protocol.AngleToByte(s.Yaw), protocol.AngleToByte(s.Pitch)
	moved := dx != 0 || dy != 0 || dz != 0
	rotated := yaw != t.yaw || pitch != t.pitch
	tooFar := !fitsRelativeMove(dx) || !fitsRelativeMove(dy) || !fitsRelativeMove(dz)

	t.sinceSync++

	switch {
	case moved && (tooFar || t.sinceSync >= positionSyncInterval):
		out = append(out, &packet.EntityPositionSync{
			EntityID: id, X: s.X, Y: s.Y, Z: s.Z, Yaw: s.Yaw, Pitch: s.Pitch, OnGround: s.OnGround,
		})
		t.sinceSync = 0
	case moved && rotated:
		out = append(out, &packet.MoveEntityPosRot{
			EntityID: id, DX: int16(dx), DY: int16(dy), DZ: int16(dz), Yaw: s.Yaw, Pitch: s.Pitch, OnGround: s.OnGround,
		})
	case moved:
		out = append(out, &packet.MoveEntityPos{EntityID: id, DX: int16(dx), DY: int16(dy), DZ: int16(dz), OnGround: s.OnGround})
	case rotated || s.OnGround != t.onGround:
		out = append(out, &packet.MoveEntityRot{EntityID: id, Yaw: s.Yaw, Pitch: s.Pitch, OnGround: s.OnGround})
	}

	if protocol.AngleToByte(s.HeadYaw) != t.headYaw {
		out = append(out, &packet.RotateHead{EntityID: id, HeadYaw: s.HeadYaw})
	}

	var changed []packet.EntityDataValue

	for _, v := range s.Metadata {
		if t.metadata[v.Index] != v {
			changed = append(changed, v)
			t.metadata[v.Index] = v
		}
	}

	if changed != nil {
		out = append(out, &packet.SetEntityData{EntityID: id, Values: changed})
	}

	t.record(s)

	return out
}

func fitsRelativeMove(d int64) bool {
	return d >= -maxRelativeMove && d <= maxRelativeMove
}
