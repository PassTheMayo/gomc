package server

import (
	"encoding/json"
	"fmt"

	"main/src/packet"
	"main/src/text"
	"main/src/version"
)

// handshake reads the opening packet and moves to the requested state.
func (s *session) handshake() (int32, error) {
	p, err := s.read(false)

	if err != nil {
		return 0, err
	}

	intention, ok := p.(*packet.Intention)

	if !ok {
		return 0, fmt.Errorf("expected handshake, got %T", p)
	}

	switch intention.Intent {
	case packet.IntentStatus:
		s.setState(stateStatus, packet.StatusServerbound, packet.StatusClientbound)
	case packet.IntentLogin, packet.IntentTransfer:
		s.setState(stateLogin, packet.LoginServerbound, packet.LoginClientbound)

		if intention.ProtocolVersion != version.Protocol {
			s.disconnect(text.Translatable("multiplayer.disconnect.incompatible", text.Text(version.Name)))

			return 0, fmt.Errorf("unsupported protocol version %d", intention.ProtocolVersion)
		}
	default:
		return 0, fmt.Errorf("unknown intent %d", intention.Intent)
	}

	return intention.Intent, nil
}

// statusResponse is the server list entry.
type statusResponse struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int            `json:"max"`
		Online int            `json:"online"`
		Sample []statusPlayer `json:"sample,omitempty"`
	} `json:"players"`
	Description        text.Component `json:"description"`
	EnforcesSecureChat bool           `json:"enforcesSecureChat"`
}

type statusPlayer struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// maxStatusSample is how many player names the server list shows.
const maxStatusSample = 12

// status answers server list pings.
func (s *session) status() {
	for {
		p, err := s.read(false)

		if err != nil {
			return
		}

		switch p := p.(type) {
		case *packet.StatusRequest:
			s.send(&packet.StatusResponse{JSON: s.srv.statusJSON()})
		case *packet.PingRequest:
			s.send(&packet.PongResponse{Payload: p.Payload})
			return
		}
	}
}

func (s *Server) statusJSON() string {
	var res statusResponse

	res.Version.Name = version.Name
	res.Version.Protocol = version.Protocol
	res.Players.Max = s.conf.MaxPlayers
	res.Description = text.Text(s.conf.Motd)

	for _, p := range s.onlinePlayers() {
		res.Players.Online++

		if len(res.Players.Sample) < maxStatusSample {
			res.Players.Sample = append(res.Players.Sample, statusPlayer{Name: p.Profile.Name, ID: p.Profile.ID.String()})
		}
	}

	data, _ := json.Marshal(res)

	return string(data)
}
