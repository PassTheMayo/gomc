// Package server implements the Minecraft server: connection handling for
// every protocol state, player sessions and the game loop.
package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"main/src/auth"
	"main/src/block"
	"main/src/command"
	"main/src/config"
	"main/src/entity"
	"main/src/packet"
	"main/src/storage"
	"main/src/text"
	"main/src/uuid"
	"main/src/world"
	"main/src/worldgen"
)

// Brand is the server software name shown in the client's debug screen.
const Brand = "go-minecraft"

// Server is a running Minecraft server.
type Server struct {
	conf       *config.Config
	log        *slog.Logger
	store      *storage.Store
	world      *world.World
	level      storage.LevelData
	gameTime   atomic.Int64
	gameMode   int32
	spawn      storage.PlayerData
	hashedSeed int64
	favicon    []byte
	links      []packet.ServerLink
	keys       *auth.KeyPair
	sessions   *auth.SessionClient
	sessionID  uuid.UUID

	router   router
	ticks    scheduler
	entities *entity.Tracker
	commands command.Dispatcher
	pregenMu sync.Mutex
	pregen   *pregenJob

	listener     net.Listener
	nextEntityID atomic.Int32
	quit         chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup

	joinMu  sync.Mutex
	mu      sync.RWMutex
	players map[uuid.UUID]*Player
	conns   map[*session]struct{}
}

// New prepares a server from its configuration: it opens the world, picks the
// spawn point and, in online mode, generates the RSA key pair.
func New(conf *config.Config, log *slog.Logger) (*Server, error) {
	gameMode, ok := packet.ParseGameMode(conf.GameMode)

	if !ok {
		return nil, fmt.Errorf("server: unknown game mode %q", conf.GameMode)
	}

	store, err := storage.Open(conf.World.Directory)

	if err != nil {
		return nil, err
	}

	level, found, err := store.LoadLevel()

	if err != nil {
		store.Close()
		return nil, err
	}

	if !found {
		level = storage.LevelData{Seed: conf.World.Seed, Generator: conf.World.Generator}

		if level.Seed == 0 {
			var b [8]byte
			_, _ = rand.Read(b[:])
			level.Seed = int64(binary.BigEndian.Uint64(b[:]))
		}

		if err = store.SaveLevel(level); err != nil {
			store.Close()
			return nil, err
		}
	} else if level.Generator != conf.World.Generator || (conf.World.Seed != 0 && level.Seed != conf.World.Seed) {
		log.Warn("The world was created with a different seed or generator, keeping the saved ones",
			"seed", level.Seed, "generator", level.Generator)
	}

	gen, err := worldgen.New(level.Generator, level.Seed, world.Overworld)

	if err != nil {
		store.Close()
		return nil, err
	}

	links, err := serverLinks(conf.ServerLinks)

	if err != nil {
		store.Close()
		return nil, err
	}

	// Like vanilla, a broken icon is logged rather than stopping the server.
	favicon, err := loadFavicon(conf.World.Directory)

	if err != nil {
		log.Error("Couldn't load the server icon", "error", err)
	}

	s := &Server{
		conf:       conf,
		log:        log,
		store:      store,
		world:      world.New(world.Overworld, gen, store, log),
		level:      level,
		gameMode:   gameMode,
		hashedSeed: hashSeed(level.Seed),
		favicon:    favicon,
		links:      links,
		sessions:   auth.NewSessionClient(),
		sessionID:  uuid.New(),
		quit:       make(chan struct{}),
		players:    make(map[uuid.UUID]*Player),
		conns:      make(map[*session]struct{}),
		entities:   entity.NewTracker(),
	}

	s.gameTime.Store(level.GameTime)
	s.world.BlockChanged = s.broadcastBlockChange
	s.spawn = s.findSpawn()
	s.registerFeatures()

	if conf.OnlineMode {
		if s.keys, err = auth.GenerateKeyPair(); err != nil {
			store.Close()
			return nil, err
		}
	}

	return s, nil
}

// registerFeatures lets every feature register its packet handlers, join
// steps, scheduled tasks and commands. Join steps and tasks run in this order.
func (s *Server) registerFeatures() {
	for _, register := range []func(){
		s.registerCommands,
		s.registerInventory,
		s.registerMovement,
		s.registerGameTime,
		s.registerEntityTracking,
		s.registerKeepAlive,
		s.registerTabList,
		s.registerServerData,
		s.registerClientInformation,
		s.registerChunkStreaming,
		s.registerBuilding,
		s.registerArmSwing,
		s.registerGameModeCommand,
		s.registerListCommand,
		s.registerPregen,
		s.registerAutosave,
	} {
		register()
	}
}

// hashSeed obfuscates the world seed for the client, which only uses it to
// smooth biome borders: the first 8 bytes of SHA-256 of the little-endian seed.
func hashSeed(seed int64) int64 {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(seed))
	sum := sha256.Sum256(b[:])

	return int64(binary.LittleEndian.Uint64(sum[:8]))
}

// findSpawn places the spawn on top of the highest solid block of the
// generator's spawn column, so players never spawn inside terrain or trees.
func (s *Server) findSpawn() storage.PlayerData {
	x, z := s.world.Generator().Spawn()
	pos := world.ChunkPosAt(float64(x), float64(z))
	c := s.world.Acquire(pos)
	defer s.world.Release(pos)

	lx, lz := x&15, z&15
	y := s.world.Dim.MaxY()

	for y > s.world.Dim.MinY && !block.BlocksMotion(c.Block(lx, y, lz)) {
		y--
	}

	return storage.PlayerData{X: float64(x) + 0.5, Y: float64(y + 1), Z: float64(z) + 0.5}
}

// ListenAndServe accepts connections until Shutdown is called.
func (s *Server) ListenAndServe() error {
	addr := net.JoinHostPort(s.conf.Host, fmt.Sprint(s.conf.Port))
	ln, err := net.Listen("tcp", addr)

	if err != nil {
		return err
	}

	return s.Serve(ln)
}

// Serve accepts connections on ln until Shutdown is called.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.log.Info("Listening for connections", "address", ln.Addr().String(), "seed", s.level.Seed,
		"generator", s.level.Generator, "online_mode", s.conf.OnlineMode)

	s.wg.Add(1)
	go s.tickLoop()

	for {
		raw, err := ln.Accept()

		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
			}

			var ne net.Error

			if errors.As(err, &ne) && ne.Timeout() {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			return err
		}

		sess := newSession(s, raw)

		s.mu.Lock()
		s.conns[sess] = struct{}{}
		s.mu.Unlock()

		s.wg.Add(1)

		go func() {
			defer s.wg.Done()
			sess.run()

			s.mu.Lock()
			delete(s.conns, sess)
			s.mu.Unlock()
		}()
	}
}

// Shutdown disconnects every player, saves the world and closes the server.
func (s *Server) Shutdown() error {
	var err error

	s.shutdownOnce.Do(func() {
		close(s.quit)

		s.mu.RLock()

		if s.listener != nil {
			s.listener.Close()
		}

		for sess := range s.conns {
			sess.disconnect(text.Translatable("multiplayer.disconnect.server_shutdown"))
		}

		s.mu.RUnlock()
		s.wg.Wait()

		s.level.GameTime = s.gameTime.Load()

		err = errors.Join(s.world.Save(), s.store.SaveLevel(s.level), s.store.Close())
		s.log.Info("Saved the world and stopped the server")
	})

	return err
}

// isOperator reports whether the named player may use restricted commands.
func (s *Server) isOperator(name string) bool {
	for _, op := range s.conf.Operators {
		if strings.EqualFold(strings.TrimSpace(op), name) {
			return true
		}
	}

	return false
}

// onlinePlayers returns a snapshot of the players in the world.
func (s *Server) onlinePlayers() []*Player {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Player, 0, len(s.players))

	for _, p := range s.players {
		out = append(out, p)
	}

	return out
}

// playerByName returns the joined player with a name, ignoring case like
// vanilla, or nil if there is none.
func (s *Server) playerByName(name string) *Player {
	for _, p := range s.joinedPlayers() {
		if strings.EqualFold(p.Profile.Name, name) {
			return p
		}
	}

	return nil
}

// viewers returns the joined players whose view includes a chunk.
func (s *Server) viewers(chunk world.ChunkPos) []*Player {
	var out []*Player

	for _, p := range s.joinedPlayers() {
		if snap := p.snapshot(); world.InView(snap.chunkPos(), snap.ViewDistance, chunk) {
			out = append(out, p)
		}
	}

	return out
}

// broadcast sends a packet to every player in the world.
func (s *Server) broadcast(p packet.Packet) {
	for _, player := range s.onlinePlayers() {
		player.Send(p)
	}
}
