package packet

import "main/src/protocol"

// BlockPos is a block position.
type BlockPos struct {
	X, Y, Z int32
}

func (p *BlockPos) encode(w *protocol.Writer) { w.Position(p.X, p.Y, p.Z) }
func (p *BlockPos) decode(r *protocol.Reader) { p.X, p.Y, p.Z = r.Position() }

// Player actions sent in PlayerAction.Action.
const (
	ActionStartDestroyBlock int32 = 0
	ActionAbortDestroyBlock int32 = 2
	ActionStopDestroyBlock  int32 = 3
	ActionDropAllItems      int32 = 4
	ActionDropItem          int32 = 5
)

// PlayerAction reports digging and a few other actions. Sequence numbers the
// block changes the client predicted, for BlockChangedAck.
type PlayerAction struct {
	Action   int32
	Pos      BlockPos
	Face     uint8
	Sequence int32
}

func (p *PlayerAction) Encode(w *protocol.Writer) {
	w.VarInt(p.Action)
	p.Pos.encode(w)
	w.UByte(p.Face)
	w.VarInt(p.Sequence)
}

func (p *PlayerAction) Decode(r *protocol.Reader) {
	p.Action = r.VarInt()
	p.Pos.decode(r)
	p.Face = r.UByte()
	p.Sequence = r.VarInt()
}

// Hands that can use items.
const (
	HandMain int32 = 0
	HandOff  int32 = 1
)

// UseItemOn reports a right click on a block face. The cursor is the clicked
// point relative to the block's minimum corner.
type UseItemOn struct {
	Hand                      int32
	Pos                       BlockPos
	Face                      int32
	CursorX, CursorY, CursorZ float32
	Inside, WorldBorderHit    bool
	Sequence                  int32
}

func (p *UseItemOn) Encode(w *protocol.Writer) {
	w.VarInt(p.Hand)
	p.Pos.encode(w)
	w.VarInt(p.Face)
	w.Float(p.CursorX)
	w.Float(p.CursorY)
	w.Float(p.CursorZ)
	w.Bool(p.Inside)
	w.Bool(p.WorldBorderHit)
	w.VarInt(p.Sequence)
}

func (p *UseItemOn) Decode(r *protocol.Reader) {
	p.Hand = r.VarInt()
	p.Pos.decode(r)
	p.Face = r.VarInt()
	p.CursorX = r.Float()
	p.CursorY = r.Float()
	p.CursorZ = r.Float()
	p.Inside = r.Bool()
	p.WorldBorderHit = r.Bool()
	p.Sequence = r.VarInt()
}

// BlockUpdate changes one block on the client.
type BlockUpdate struct {
	Pos   BlockPos
	State int32
}

func (p *BlockUpdate) Encode(w *protocol.Writer) {
	p.Pos.encode(w)
	w.VarInt(p.State)
}

func (p *BlockUpdate) Decode(r *protocol.Reader) {
	p.Pos.decode(r)
	p.State = r.VarInt()
}

// BlockChangedAck confirms the client's predicted block changes up to Sequence.
type BlockChangedAck struct {
	Sequence int32
}

func (p *BlockChangedAck) Encode(w *protocol.Writer) { w.VarInt(p.Sequence) }
func (p *BlockChangedAck) Decode(r *protocol.Reader) { p.Sequence = r.VarInt() }
