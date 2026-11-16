package server

import (
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"main/src/conn"
	"main/src/packet"
	"main/src/profile"
	"main/src/text"
)

// Connection states.
const (
	stateHandshake int32 = iota
	stateStatus
	stateLogin
	stateConfiguration
	statePlay
)

const (
	// handshakeTimeout bounds how long a connection may take to log in.
	handshakeTimeout = 30 * time.Second

	// readTimeout disconnects clients that stop sending packets in play,
	// which they never do while connected because of keep-alives.
	readTimeout = 60 * time.Second

	// sendQueueSize is how many packets may wait for a slow client before it
	// is disconnected.
	sendQueueSize = 4096
)

type outgoing struct {
	id      int32
	payload []byte
	close   bool
}

// session is one client connection, from handshake to disconnect.
type session struct {
	srv  *Server
	conn *conn.Conn
	log  *slog.Logger

	state    atomic.Int32
	tableMu  sync.RWMutex
	inbound  *packet.Table
	outbound *packet.Table

	queue     chan outgoing
	queued    atomic.Bool
	closed    chan struct{}
	closeOnce sync.Once

	profile    profile.GameProfile
	clientInfo packet.ClientInformation
	player     *Player
}

func newSession(srv *Server, raw net.Conn) *session {
	s := &session{
		srv:     srv,
		conn:    conn.New(raw),
		log:     srv.log.With("remote", raw.RemoteAddr().String()),
		inbound: packet.HandshakeServerbound,
		closed:  make(chan struct{}),
	}

	return s
}

// setState switches the packet tables used for reading and writing.
func (s *session) setState(state int32, inbound, outbound *packet.Table) {
	s.tableMu.Lock()
	s.inbound, s.outbound = inbound, outbound
	s.tableMu.Unlock()

	s.state.Store(state)
}

func (s *session) tables() (inbound, outbound *packet.Table) {
	s.tableMu.RLock()
	defer s.tableMu.RUnlock()

	return s.inbound, s.outbound
}

func (s *session) run() {
	defer s.close()

	_ = s.conn.SetReadDeadline(time.Now().Add(handshakeTimeout))

	intent, err := s.handshake()

	if err != nil {
		s.log.Debug("Handshake failed", "error", err)
		return
	}

	switch intent {
	case packet.IntentStatus:
		s.status()
	case packet.IntentLogin, packet.IntentTransfer:
		if !s.login() || !s.configure() {
			return
		}

		_ = s.conn.SetReadDeadline(time.Time{})
		s.play()
	}
}

// read returns the next packet known to the current state. Unknown packets
// are skipped when skipUnknown is set, which lets the server ignore the many
// play packets it does not implement.
func (s *session) read(skipUnknown bool) (packet.Packet, error) {
	for {
		id, payload, err := s.conn.ReadPacket()

		if err != nil {
			return nil, err
		}

		inbound, _ := s.tables()
		p, err := inbound.Decode(id, payload)

		if errors.Is(err, packet.ErrUnknownPacket) && skipUnknown {
			continue
		}

		return p, err
	}
}

// send writes a packet using the table of the current state. Once the session
// is in the play state packets are queued and written by a separate goroutine,
// so game logic never blocks on a slow client.
func (s *session) send(p packet.Packet) {
	_, outbound := s.tables()
	id := outbound.ID(p)
	payload := packet.Encode(p)

	if !s.queued.Load() {
		if err := s.conn.WritePacket(id, payload); err != nil {
			s.close()
		}

		return
	}

	s.enqueue(outgoing{id: id, payload: payload})
}

func (s *session) enqueue(o outgoing) {
	select {
	case <-s.closed:
	case s.queue <- o:
	default:
		s.log.Warn("Disconnecting client that cannot keep up with outgoing packets")
		s.close()
	}
}

// startQueue switches the session to asynchronous writes.
func (s *session) startQueue() {
	s.queue = make(chan outgoing, sendQueueSize)
	s.queued.Store(true)

	go func() {
		for {
			select {
			case <-s.closed:
				return
			case o := <-s.queue:
				if o.close {
					s.close()
					return
				}

				if err := s.conn.WritePacket(o.id, o.payload); err != nil {
					s.close()
					return
				}
			}
		}
	}()
}

// disconnect tells the client why it is being disconnected, then closes the
// connection once the message has been written.
func (s *session) disconnect(reason text.Component) {
	s.log.Info("Disconnecting client", "reason", reason.PlainText())

	switch s.state.Load() {
	case stateLogin:
		s.send(&packet.LoginDisconnect{Reason: reason})
	case stateConfiguration, statePlay:
		s.send(&packet.Disconnect{Reason: reason})
	}

	if s.queued.Load() {
		s.enqueue(outgoing{close: true})
	} else {
		s.close()
	}
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.conn.Close()
	})
}
