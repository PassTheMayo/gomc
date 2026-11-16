package server

import (
	"time"

	"main/src/packet"
	"main/src/text"
)

const (
	// keepAliveInterval is how often the server pings each client.
	keepAliveInterval = 15 * time.Second

	// keepAliveTimeout is how long a client may take to answer a ping.
	keepAliveTimeout = 30 * time.Second
)

// registerKeepAlive pings clients and handles their answers.
func (s *Server) registerKeepAlive() {
	on(&s.router, func(p *Player, pk *packet.KeepAlive) { p.keepAliveResponse(pk.ID) })

	s.ticks.every(1, func(t *tick) {
		for _, p := range t.players {
			p.tickKeepAlive(t.now)
		}
	})
}

// tickKeepAlive pings the client and disconnects it if a previous ping went
// unanswered for too long.
func (p *Player) tickKeepAlive(now time.Time) {
	p.mu.Lock()
	pending, sent := p.keepAliveID != 0, p.keepAliveSent
	due := !pending && now.Sub(sent) >= keepAliveInterval

	if due {
		p.keepAliveID = now.UnixMilli()
		p.keepAliveSent = now
	}

	id := p.keepAliveID
	p.mu.Unlock()

	switch {
	case pending && now.Sub(sent) > keepAliveTimeout:
		p.session.disconnect(text.Translatable("disconnect.timeout"))
	case due:
		p.Send(&packet.KeepAlive{ID: id})
	}
}

// keepAliveResponse records the round trip time of an answered ping, smoothed
// like the vanilla server does.
func (p *Player) keepAliveResponse(id int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if id != p.keepAliveID || id == 0 {
		return
	}

	rtt := int32(time.Since(p.keepAliveSent).Milliseconds())
	p.latency = (p.latency*3 + rtt) / 4
	p.keepAliveID = 0
}
