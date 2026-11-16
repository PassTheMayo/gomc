package packet

import (
	"main/src/item"
	"main/src/protocol"
)

// Punch reports a left click: the player swings its main hand.
type Punch struct{}

func (p *Punch) Encode(*protocol.Writer) {}
func (p *Punch) Decode(*protocol.Reader) {}

// SwingAnimation shows an entity swinging one of its arms.
type SwingAnimation struct {
	EntityID  int32
	Hand      int32
	Animation item.SwingAnimation
}

func (p *SwingAnimation) Encode(w *protocol.Writer) {
	w.VarInt(p.EntityID)
	w.VarInt(p.Hand)
	w.VarInt(p.Animation.Type)
	w.VarInt(p.Animation.Duration)
}

func (p *SwingAnimation) Decode(r *protocol.Reader) {
	p.EntityID = r.VarInt()
	p.Hand = r.VarInt()
	p.Animation.Type = r.VarInt()
	p.Animation.Duration = r.VarInt()
}
