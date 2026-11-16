package entity

import (
	"testing"

	"main/src/packet"
	"main/src/uuid"
	"main/src/world"
)

type fakeEntity struct {
	id    int32
	state State
}

func (e *fakeEntity) ID() int32       { return e.id }
func (e *fakeEntity) UUID() uuid.UUID { return uuid.UUID{} }
func (e *fakeEntity) Type() int32     { return 7 }
func (e *fakeEntity) State() State    { return e.state }

type fakeViewer struct {
	id   int32
	sent []packet.Packet
}

func (v *fakeViewer) ID() int32                   { return v.id }
func (v *fakeViewer) View() (world.ChunkPos, int) { return world.ChunkPos{}, 2 }
func (v *fakeViewer) Send(pk packet.Packet)       { v.sent = append(v.sent, pk) }
func (v *fakeViewer) take() (out []packet.Packet) { out, v.sent = v.sent, nil; return out }

func TestTracker(t *testing.T) {
	sneaking := packet.EntityDataValue{Index: packet.DataPose, Type: packet.DataTypePose, Pose: packet.PoseCrouching}
	e := &fakeEntity{id: 1, state: State{X: 8, Y: 64, Z: 8}}
	v := &fakeViewer{id: 2}
	tr := NewTracker()

	// An entity in view spawns, and a viewer never sees itself.
	tr.Update([]Entity{e, &fakeEntity{id: 2}}, []Viewer{v})

	if sent := v.take(); len(sent) != 1 {
		t.Fatalf("first update sent %#v, want one spawn packet", sent)
	} else if add, ok := sent[0].(*packet.AddEntity); !ok || add.EntityID != 1 || add.Type != 7 {
		t.Fatalf("spawned %#v", sent[0])
	}

	// Small moves are relative; changed metadata is sent on its own.
	e.state.X += 0.5
	e.state.Metadata = []packet.EntityDataValue{sneaking}
	tr.Update([]Entity{e}, []Viewer{v})
	sent := v.take()

	if move, ok := sent[0].(*packet.MoveEntityPos); !ok || move.DX != 2048 {
		t.Fatalf("moving sent %#v", sent[0])
	}

	if data, ok := sent[1].(*packet.SetEntityData); !ok || !data.Values[0].Equal(sneaking) {
		t.Fatalf("metadata change sent %#v", sent[1])
	}

	// Nothing is sent while nothing changes.
	tr.Update([]Entity{e}, []Viewer{v})

	if sent := v.take(); sent != nil {
		t.Fatalf("an unchanged entity sent %#v", sent)
	}

	// Moves too large for a relative move are absolute.
	e.state.X += 10
	tr.Update([]Entity{e}, []Viewer{v})

	if sent := v.take(); len(sent) != 1 {
		t.Fatalf("a long move sent %#v", sent)
	} else if _, ok := sent[0].(*packet.EntityPositionSync); !ok {
		t.Fatalf("a long move sent %#v", sent[0])
	}

	// Leaving the view or disappearing removes the entity.
	e.state.X = 1000
	tr.Update([]Entity{e}, []Viewer{v})

	if sent := v.take(); len(sent) != 1 {
		t.Fatalf("leaving the view sent %#v", sent)
	} else if rm, ok := sent[0].(*packet.RemoveEntities); !ok || rm.EntityIDs[0] != 1 {
		t.Fatalf("leaving the view sent %#v", sent[0])
	}
}

func TestSendToViewers(t *testing.T) {
	near := &fakeEntity{id: 1, state: State{X: 8, Z: 8}}
	far := &fakeEntity{id: 3, state: State{X: 1000, Z: 1000}}
	v := &fakeViewer{id: 2}
	tr := NewTracker()
	tr.Update([]Entity{near, far}, []Viewer{v})
	v.take()

	tr.SendToViewers(near.id, &packet.SwingAnimation{EntityID: near.id})
	tr.SendToViewers(far.id, &packet.SwingAnimation{EntityID: far.id})

	if sent := v.take(); len(sent) != 1 || sent[0].(*packet.SwingAnimation).EntityID != near.id {
		t.Fatalf("viewers were sent %#v, want only the visible entity's packet", sent)
	}

	// Viewers that are gone receive nothing.
	tr.Update([]Entity{near}, nil)
	tr.SendToViewers(near.id, &packet.SwingAnimation{EntityID: near.id})

	if sent := v.take(); sent != nil {
		t.Fatalf("a viewer that left was sent %#v", sent)
	}
}
