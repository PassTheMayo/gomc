package packet

import "main/src/protocol"

// Client intents sent in the handshake.
const (
	IntentStatus   int32 = 1
	IntentLogin    int32 = 2
	IntentTransfer int32 = 3
)

// Intention opens a connection and selects the next state.
type Intention struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	Intent          int32
}

func (p *Intention) Encode(w *protocol.Writer) {
	w.VarInt(p.ProtocolVersion)
	w.String(p.ServerAddress)
	w.UShort(p.ServerPort)
	w.VarInt(p.Intent)
}

func (p *Intention) Decode(r *protocol.Reader) {
	p.ProtocolVersion = r.VarInt()
	p.ServerAddress = r.String(255)
	p.ServerPort = r.UShort()
	p.Intent = r.VarInt()
}
