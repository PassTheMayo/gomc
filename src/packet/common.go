package packet

import (
	"main/src/protocol"
	"main/src/text"
)

// KeepAlive is exchanged periodically in both directions to detect dead
// connections.
type KeepAlive struct {
	ID int64
}

func (p *KeepAlive) Encode(w *protocol.Writer) { w.Long(p.ID) }
func (p *KeepAlive) Decode(r *protocol.Reader) { p.ID = r.Long() }

// Disconnect closes the connection with a reason shown to the player.
type Disconnect struct {
	Reason text.Component
}

func (p *Disconnect) Encode(w *protocol.Writer) { writeComponent(w, p.Reason) }
func (p *Disconnect) Decode(r *protocol.Reader) { p.Reason = readComponent(r) }

// CustomPayload carries plugin channel data, such as the server brand.
type CustomPayload struct {
	Channel string
	Data    []byte
}

func (p *CustomPayload) Encode(w *protocol.Writer) {
	w.String(p.Channel)
	w.Raw(p.Data)
}

func (p *CustomPayload) Decode(r *protocol.Reader) {
	p.Channel = r.String(32767)
	p.Data = r.Rest()
}

// ClientInformation reports the player's client settings. ModelCustomisation
// is a bit mask of the visible skin layers.
type ClientInformation struct {
	Locale             string
	ViewDistance       int8
	ChatVisibility     int32
	ChatColors         bool
	ModelCustomisation uint8
	MainHand           int32
	TextFiltering      bool
	AllowsListing      bool
	ParticleStatus     int32
}

func (p *ClientInformation) Encode(w *protocol.Writer) {
	w.String(p.Locale)
	w.Byte(p.ViewDistance)
	w.VarInt(p.ChatVisibility)
	w.Bool(p.ChatColors)
	w.UByte(p.ModelCustomisation)
	w.VarInt(p.MainHand)
	w.Bool(p.TextFiltering)
	w.Bool(p.AllowsListing)
	w.VarInt(p.ParticleStatus)
}

func (p *ClientInformation) Decode(r *protocol.Reader) {
	p.Locale = r.String(16)
	p.ViewDistance = r.Byte()
	p.ChatVisibility = r.VarInt()
	p.ChatColors = r.Bool()
	p.ModelCustomisation = r.UByte()
	p.MainHand = r.VarInt()
	p.TextFiltering = r.Bool()
	p.AllowsListing = r.Bool()
	p.ParticleStatus = r.VarInt()
}
