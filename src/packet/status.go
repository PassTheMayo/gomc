package packet

import "main/src/protocol"

// StatusRequest asks for the server list entry.
type StatusRequest struct{}

func (p *StatusRequest) Encode(*protocol.Writer) {}
func (p *StatusRequest) Decode(*protocol.Reader) {}

// StatusResponse carries the server list entry as JSON.
type StatusResponse struct {
	JSON string
}

func (p *StatusResponse) Encode(w *protocol.Writer) { w.String(p.JSON) }
func (p *StatusResponse) Decode(r *protocol.Reader) { p.JSON = r.String(32767) }

// PingRequest measures latency in the status and play states.
type PingRequest struct {
	Payload int64
}

func (p *PingRequest) Encode(w *protocol.Writer) { w.Long(p.Payload) }
func (p *PingRequest) Decode(r *protocol.Reader) { p.Payload = r.Long() }

// PongResponse echoes a PingRequest payload.
type PongResponse struct {
	Payload int64
}

func (p *PongResponse) Encode(w *protocol.Writer) { w.Long(p.Payload) }
func (p *PongResponse) Decode(r *protocol.Reader) { p.Payload = r.Long() }
