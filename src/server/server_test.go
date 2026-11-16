package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"main/src/auth"
	"main/src/block"
	"main/src/config"
	"main/src/conn"
	"main/src/inventory"
	"main/src/item"
	"main/src/packet"
	"main/src/protocol"
	"main/src/registry"
	"main/src/text"
	"main/src/uuid"
	"main/src/version"
	"main/src/world"
	"main/src/worldgen"
)

func testConfig(dir string) *config.Config {
	conf := config.New()
	conf.Host = "127.0.0.1"
	conf.Motd = "Test server"
	conf.MaxPlayers = 10
	conf.CompressionThreshold = 256
	conf.ViewDistance = 3
	conf.SimulationDistance = 3
	conf.GameMode = "creative"
	conf.World.Directory = dir
	conf.World.Seed = 1
	conf.World.Generator = worldgen.NameTerrain

	return conf
}

func startServer(t *testing.T, conf *config.Config) (*Server, string) {
	t.Helper()

	srv, err := New(conf, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")

	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = srv.Serve(ln) }()

	t.Cleanup(func() { _ = srv.Shutdown() })

	return srv, ln.Addr().String()
}

// stopWhenEmpty waits for every player to leave, so their data is saved, then
// shuts the server down.
func stopWhenEmpty(t *testing.T, srv *Server) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for srv.playerCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if err := srv.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

// testClient is a minimal Minecraft client driving the server under test.
type testClient struct {
	t          *testing.T
	conn       *conn.Conn
	in, out    *packet.Table
	chunks     map[world.ChunkPos]*packet.LevelChunkWithLight
	pos        [3]float64
	entityID   int32
	teleported bool
	links      *packet.ServerLinks
}

func dial(t *testing.T, addr string) *testClient {
	t.Helper()

	raw, err := net.Dial("tcp", addr)

	if err != nil {
		t.Fatal(err)
	}

	c := &testClient{t: t, conn: conn.New(raw), chunks: make(map[world.ChunkPos]*packet.LevelChunkWithLight)}
	t.Cleanup(func() { c.conn.Close() })

	return c
}

func (c *testClient) send(p packet.Packet) {
	c.t.Helper()

	if err := c.conn.WritePacket(c.out.ID(p), packet.Encode(p)); err != nil {
		c.t.Fatalf("sending %T: %v", p, err)
	}
}

// next reads one packet, answering keep-alives, teleports and chunk batches
// the way the vanilla client does.
func (c *testClient) next() packet.Packet {
	c.t.Helper()

	_ = c.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	id, payload, err := c.conn.ReadPacket()

	if err != nil {
		c.t.Fatalf("reading packet: %v", err)
	}

	p, err := c.in.Decode(id, payload)

	if err != nil {
		c.t.Fatalf("server sent an undecodable packet: %v", err)
	}

	switch p := p.(type) {
	case *packet.KeepAlive:
		if c.in == packet.PlayClientbound {
			c.send(&packet.KeepAlive{ID: p.ID})
		}
	case *packet.PlayerPosition:
		c.pos = [3]float64{p.X, p.Y, p.Z}
		c.teleported = true
		c.send(&packet.AcceptTeleportation{TeleportID: p.TeleportID, X: p.X, Y: p.Y, Z: p.Z, Yaw: p.Yaw, Pitch: p.Pitch})
	case *packet.LevelChunkWithLight:
		c.chunks[world.ChunkPos{X: p.X, Z: p.Z}] = p
	case *packet.ChunkBatchFinished:
		c.send(&packet.ChunkBatchReceived{DesiredChunksPerTick: 25})
	}

	return p
}

// waitFor reads packets until one of type T matches.
func waitFor[T packet.Packet](c *testClient, match func(T) bool) T {
	c.t.Helper()

	for {
		if p, ok := c.next().(T); ok && (match == nil || match(p)) {
			return p
		}
	}
}

func (c *testClient) handshake(intent int32, protocolVersion int32) {
	c.out = packet.HandshakeServerbound
	c.send(&packet.Intention{ProtocolVersion: protocolVersion, ServerAddress: "localhost", ServerPort: 25565, Intent: intent})

	if intent == packet.IntentStatus {
		c.in, c.out = packet.StatusClientbound, packet.StatusServerbound
	} else {
		c.in, c.out = packet.LoginClientbound, packet.LoginServerbound
	}
}

// login performs an offline login and the configuration phase, returning the
// Login packet that starts the play state.
func (c *testClient) login(name string) *packet.Login {
	c.t.Helper()

	c.handshake(packet.IntentLogin, version.Protocol)
	c.send(&packet.LoginHello{Name: name, UUID: uuid.OfflinePlayer(name)})
	c.finishLogin()

	return c.configure()
}

func (c *testClient) finishLogin() *packet.LoginFinished {
	c.t.Helper()

	for {
		switch p := c.next().(type) {
		case *packet.LoginCompression:
			c.conn.SetCompression(int(p.Threshold))
		case *packet.LoginFinished:
			c.send(&packet.LoginAcknowledged{})
			c.in, c.out = packet.ConfigurationClientbound, packet.ConfigurationServerbound

			return p
		case *packet.LoginDisconnect:
			c.t.Fatalf("login rejected: %s", p.Reason.PlainText())
		}
	}
}

func (c *testClient) configure() *packet.Login {
	c.t.Helper()

	c.send(&packet.ClientInformation{Locale: "en_us", ViewDistance: 3, ChatColors: true, ModelCustomisation: 0x7F, MainHand: 1})

	registries := 0
	var tags *packet.UpdateTags

	for {
		switch p := c.next().(type) {
		case *packet.SelectKnownPacks:
			if len(p.Packs) != 1 || p.Packs[0] != corePack {
				c.t.Fatalf("unexpected known packs %+v", p.Packs)
			}

			c.send(&packet.SelectKnownPacks{Packs: p.Packs})
		case *packet.RegistryData:
			registries++
		case *packet.ServerLinks:
			c.links = p
		case *packet.UpdateTags:
			tags = p
		case *packet.FinishConfiguration:
			if registries != len(registry.Synchronized()) || tags == nil || len(tags.Tags[registry.Item]) == 0 ||
				len(tags.Tags["minecraft:timeline"]) == 0 {
				c.t.Fatalf("configuration incomplete: %d registries, tags %v", registries, tags != nil)
			}

			c.send(&packet.FinishConfiguration{})
			c.in, c.out = packet.PlayClientbound, packet.PlayServerbound

			login := waitFor[*packet.Login](c, nil)
			c.entityID = login.EntityID

			return login
		}
	}
}

// blockAt returns a block from the chunks the client received.
func (c *testClient) blockAt(x, y, z int) block.StateID {
	c.t.Helper()

	data, ok := c.chunks[world.ChunkPosAt(float64(x), float64(z))]

	if !ok {
		c.t.Fatalf("chunk containing %d,%d was not received", x, z)
	}

	r := protocol.NewReader(data.Data)
	var sections []*world.Section

	for range world.Overworld.Sections() {
		sections = append(sections, world.DecodeSection(r))
	}

	if err := r.Finish(); err != nil {
		c.t.Fatal(err)
	}

	sy := y - world.Overworld.MinY

	return sections[sy>>4].Block(x&15, sy&15, z&15)
}

func TestStatus(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))
	c := dial(t, addr)

	c.handshake(packet.IntentStatus, version.Protocol)
	c.send(&packet.StatusRequest{})

	res := waitFor[*packet.StatusResponse](c, nil)

	var status struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max int `json:"max"`
		} `json:"players"`
		Description struct {
			Text string `json:"text"`
		} `json:"description"`
	}

	if err := json.Unmarshal([]byte(res.JSON), &status); err != nil {
		t.Fatal(err)
	}

	if status.Version.Protocol != version.Protocol || status.Version.Name != version.Name ||
		status.Players.Max != 10 || status.Description.Text != "Test server" {
		t.Fatalf("unexpected status %s", res.JSON)
	}

	c.send(&packet.PingRequest{Payload: 1234})

	if pong := waitFor[*packet.PongResponse](c, nil); pong.Payload != 1234 {
		t.Fatalf("pong payload %d", pong.Payload)
	}
}

func TestJoinAndMovement(t *testing.T) {
	srv, addr := startServer(t, testConfig(t.TempDir()))
	a := dial(t, addr)
	login := a.login("Alice")

	if login.Spawn.Dimension != "minecraft:overworld" || login.Spawn.GameMode != packet.GameModeCreative ||
		login.Spawn.DimensionType != registry.MustGet(registry.DimensionType).MustID("minecraft:overworld") {
		t.Fatalf("unexpected login packet %+v", login)
	}

	// The client leaves the loading screen once its own chunk arrives.
	waitFor[*packet.GameEvent](a, func(p *packet.GameEvent) bool { return p.Event == packet.GameEventLevelChunksLoadStart })

	if !a.teleported {
		t.Fatal("the player must be teleported to its spawn position before chunks load")
	}

	spawnChunk := world.ChunkPosAt(a.pos[0], a.pos[2])

	waitFor[*packet.ChunkBatchFinished](a, func(*packet.ChunkBatchFinished) bool {
		_, ok := a.chunks[spawnChunk]
		return ok
	})

	x, y, z := int(math.Floor(a.pos[0])), int(math.Floor(a.pos[1])), int(math.Floor(a.pos[2]))

	if !block.BlocksMotion(a.blockAt(x, y-1, z)) || block.BlocksMotion(a.blockAt(x, y, z)) {
		t.Fatalf("player spawned at %v above %s inside %s", a.pos, a.blockAt(x, y-1, z), a.blockAt(x, y, z))
	}

	// Every chunk within the view distance eventually arrives.
	expected := 0

	for dx := int32(-4); dx <= 4; dx++ {
		for dz := int32(-4); dz <= 4; dz++ {
			if world.InView(spawnChunk, 3, world.ChunkPos{X: spawnChunk.X + dx, Z: spawnChunk.Z + dz}) {
				expected++
			}
		}
	}

	waitFor[*packet.ChunkBatchFinished](a, func(*packet.ChunkBatchFinished) bool { return len(a.chunks) >= expected })

	// A second player appears for the first one.
	b := dial(t, addr)
	b.login("Bob")
	bobID := uuid.OfflinePlayer("Bob")

	waitFor[*packet.PlayerInfoUpdate](a, func(p *packet.PlayerInfoUpdate) bool {
		return p.Actions&packet.InfoAddPlayer != 0 && p.Entries[0].Profile.ID == bobID
	})

	spawn := waitFor[*packet.AddEntity](a, func(p *packet.AddEntity) bool { return p.UUID == bobID })

	if spawn.Type != playerEntityType || spawn.EntityID != b.entityID {
		t.Fatalf("unexpected spawn packet %+v", spawn)
	}

	// Bob walks one block east once his teleport is confirmed.
	waitFor[*packet.PlayerPosition](b, nil)
	b.send(&packet.MovePlayerPos{X: b.pos[0] + 1, Y: b.pos[1], Z: b.pos[2], MoveFlags: packet.MoveFlags{OnGround: true}})

	move := waitFor[*packet.MoveEntityPos](a, func(p *packet.MoveEntityPos) bool { return p.EntityID == b.entityID })

	if move.DX != 4096 || move.DY != 0 || move.DZ != 0 || !move.OnGround {
		t.Fatalf("unexpected move %+v", move)
	}

	// Bob looks around, then crouches.
	b.send(&packet.MovePlayerRot{Yaw: 90, Pitch: 45, MoveFlags: packet.MoveFlags{OnGround: true}})
	waitFor[*packet.MoveEntityRot](a, func(p *packet.MoveEntityRot) bool { return p.EntityID == b.entityID && p.Yaw == 90 })
	waitFor[*packet.RotateHead](a, func(p *packet.RotateHead) bool { return p.EntityID == b.entityID && p.HeadYaw == 90 })

	b.send(&packet.PlayerInput{Flags: packet.InputSneak})
	waitFor[*packet.SetEntityData](a, func(p *packet.SetEntityData) bool {
		for _, v := range p.Values {
			if v.Index == packet.DataPose && v.Pose == packet.PoseCrouching {
				return p.EntityID == b.entityID
			}
		}

		return false
	})

	// A teleport-sized move is sent as an absolute position.
	b.send(&packet.MovePlayerPos{X: b.pos[0] + 20, Y: b.pos[1] + 3, Z: b.pos[2], MoveFlags: packet.MoveFlags{}})
	sync := waitFor[*packet.EntityPositionSync](a, func(p *packet.EntityPositionSync) bool { return p.EntityID == b.entityID })

	if sync.X != b.pos[0]+20 || sync.Y != b.pos[1]+3 || sync.OnGround {
		t.Fatalf("unexpected position sync %+v", sync)
	}

	// Bob leaves.
	b.conn.Close()

	// The tab list removal and the entity removal may arrive in either order.
	for despawned, unlisted := false, false; !despawned || !unlisted; {
		switch p := a.next().(type) {
		case *packet.RemoveEntities:
			despawned = despawned || (len(p.EntityIDs) == 1 && p.EntityIDs[0] == b.entityID)
		case *packet.PlayerInfoRemove:
			unlisted = unlisted || p.UUIDs[0] == bobID
		}
	}

	if n := srv.playerCount(); n != 1 {
		t.Fatalf("%d players online after Bob left", n)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	conf := testConfig(dir)
	conf.World.Seed = 0
	srv, addr := startServer(t, conf)

	a := dial(t, addr)
	a.login("Alice")
	waitFor[*packet.PlayerPosition](a, nil)

	target := [3]float64{a.pos[0] + 5.5, a.pos[1] + 2, a.pos[2] - 3.25}
	a.send(&packet.MovePlayerPosRot{X: target[0], Y: target[1], Z: target[2], Yaw: 45, Pitch: 10})
	waitFor[*packet.ChunkBatchFinished](a, nil)
	a.conn.Close()

	seed := srv.level.Seed
	stopWhenEmpty(t, srv)

	regions, _ := filepath.Glob(filepath.Join(dir, "regions", "*.region"))

	if len(regions) == 0 {
		t.Fatal("no region files were written")
	}

	srv2, addr2 := startServer(t, testConfig(dir))

	if srv2.level.Seed != seed {
		t.Fatalf("seed changed from %d to %d across restarts", seed, srv2.level.Seed)
	}

	b := dial(t, addr2)
	b.login("Alice")
	waitFor[*packet.PlayerPosition](b, nil)

	if b.pos != target {
		t.Fatalf("player restored at %v, want %v", b.pos, target)
	}
}

func TestDuplicateLogin(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))

	first := dial(t, addr)
	first.login("Alice")

	second := dial(t, addr)
	second.login("Alice")

	kick := waitFor[*packet.Disconnect](first, nil)

	if kick.Reason.Translate != "multiplayer.disconnect.duplicate_login" {
		t.Fatalf("unexpected disconnect reason %+v", kick.Reason)
	}
}

func TestOutdatedClient(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))
	c := dial(t, addr)

	c.handshake(packet.IntentLogin, version.Protocol-1)
	c.send(&packet.LoginHello{Name: "Alice", UUID: uuid.OfflinePlayer("Alice")})

	if p := waitFor[*packet.LoginDisconnect](c, nil); p.Reason.Translate != "multiplayer.disconnect.incompatible" {
		t.Fatalf("unexpected disconnect reason %+v", p.Reason)
	}
}

func TestOnlineModeLogin(t *testing.T) {
	steve := uuid.New()
	var expectedHash string

	sessionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("serverId") != expectedHash || r.URL.Query().Get("username") != "Steve" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         strings.ReplaceAll(steve.String(), "-", ""),
			"name":       "Steve",
			"properties": []map[string]string{{"name": "textures", "value": "skin", "signature": "signed"}},
		})
	}))
	defer sessionServer.Close()

	conf := testConfig(t.TempDir())
	conf.OnlineMode = true
	srv, addr := startServer(t, conf)
	srv.sessions.BaseURL = sessionServer.URL

	c := dial(t, addr)
	c.handshake(packet.IntentLogin, version.Protocol)
	c.send(&packet.LoginHello{Name: "Steve", UUID: steve})

	req := waitFor[*packet.EncryptionRequest](c, nil)
	pub, err := x509.ParsePKIXPublicKey(req.PublicKey)

	if err != nil || !req.ShouldAuthenticate {
		t.Fatalf("invalid encryption request: %v", err)
	}

	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	encSecret, _ := encryptPKCS1(pub.(*rsa.PublicKey), secret)
	encChallenge, _ := encryptPKCS1(pub.(*rsa.PublicKey), req.Challenge)
	expectedHash = auth.ServerHash(req.ServerID, secret, req.PublicKey)

	c.send(&packet.EncryptionResponse{SharedSecret: encSecret, Challenge: encChallenge})

	if err = c.conn.EnableEncryption(secret); err != nil {
		t.Fatal(err)
	}

	finished := c.finishLogin()

	if finished.Profile.ID != steve || len(finished.Profile.Properties) != 1 || finished.Profile.Properties[0].Signature != "signed" {
		t.Fatalf("unexpected profile %+v", finished.Profile)
	}

	if finished.SessionID != srv.sessionID {
		t.Fatal("login must carry the server session ID")
	}

	// The rest of the session runs encrypted and compressed.
	login := c.configure()

	if !login.OnlineMode {
		t.Fatal("the login packet must report online mode")
	}
}

// encryptPKCS1 encrypts like the vanilla client does during login.
func encryptPKCS1(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	//lint:ignore SA1019 the Minecraft client encrypts the shared secret with PKCS #1 v1.5.
	return rsa.EncryptPKCS1v15(rand.Reader, pub, data)
}

// rootCommand finds a top-level command in a command tree.
func rootCommand(tree *packet.Commands, name string) (packet.CommandNode, bool) {
	for _, child := range tree.Nodes[tree.Root].Children {
		if tree.Nodes[child].Name == name {
			return tree.Nodes[child], true
		}
	}

	return packet.CommandNode{}, false
}

func TestPregenCommand(t *testing.T) {
	conf := testConfig(t.TempDir())
	conf.Operators = []string{"alice"}
	srv, addr := startServer(t, conf)

	a := dial(t, addr)
	a.login("Alice")

	if pregen, ok := rootCommand(waitFor[*packet.Commands](a, nil), "pregen"); !ok || !pregen.Restricted {
		t.Fatalf("operators must see /pregen as restricted, got %+v", pregen)
	}

	b := dial(t, addr)
	b.login("Bob")

	if _, ok := rootCommand(waitFor[*packet.Commands](b, nil), "pregen"); ok {
		t.Fatal("non-operators must not see /pregen")
	}

	b.send(&packet.ChatCommand{Command: "pregen 2"})

	reply := waitFor[*packet.SystemChat](b, func(p *packet.SystemChat) bool { return p.Content.Translate != "multiplayer.player.joined" })

	if reply.Content.Translate != "command.unknown.command" {
		t.Fatalf("non-operator got %+v", reply.Content)
	}

	// The radius reaches beyond the view distance of 3, so the outer chunks
	// are generated and saved rather than skipped as loaded.
	waitFor[*packet.PlayerPosition](a, nil)
	center := world.ChunkPosAt(a.pos[0], a.pos[2])
	a.send(&packet.ChatCommand{Command: "pregen 6"})

	isChat := func(prefix string) func(*packet.SystemChat) bool {
		return func(p *packet.SystemChat) bool { return !p.Overlay && strings.HasPrefix(p.Content.Text, prefix) }
	}

	start := waitFor[*packet.SystemChat](a, isChat("Pre-generating"))

	if !strings.Contains(start.Content.Text, "169 chunks") {
		t.Fatalf("unexpected start message %q", start.Content.Text)
	}

	done := waitFor[*packet.SystemChat](a, isChat("Pre-generated"))

	if !strings.HasPrefix(done.Content.Text, "Pre-generated 169 chunks") || done.Content.Color != text.Green {
		t.Fatalf("unexpected completion message %+v", done.Content)
	}

	for _, d := range [][2]int32{{-6, -6}, {6, 6}, {-6, 6}, {6, 0}} {
		pos := world.ChunkPos{X: center.X + d[0], Z: center.Z + d[1]}

		if stored, err := srv.store.HasChunk(pos); err != nil || !stored {
			t.Fatalf("chunk %v was not saved: %v", pos, err)
		}
	}

	// Only one job runs at a time, and a running job can be cancelled.
	a.send(&packet.ChatCommand{Command: "pregen 1024"})
	waitFor[*packet.SystemChat](a, isChat("Pre-generating"))

	a.send(&packet.ChatCommand{Command: "pregen 5"})
	waitFor[*packet.SystemChat](a, isChat("A pre-generation is already running"))

	a.send(&packet.ChatCommand{Command: "pregen cancel"})
	stopped := waitFor[*packet.SystemChat](a, isChat("Pre-generation stopped"))

	if !strings.Contains(stopped.Content.Text, "of 4198401 chunks") {
		t.Fatalf("unexpected stop message %q", stopped.Content.Text)
	}

	a.send(&packet.ChatCommand{Command: "pregen cancel"})
	waitFor[*packet.SystemChat](a, isChat("No pre-generation is running"))
}

// groundAt returns the y of the highest solid block in a column the client received.
func (c *testClient) groundAt(x, z int) int {
	c.t.Helper()

	for y := world.Overworld.MaxY(); y > world.Overworld.MinY; y-- {
		if block.IsSolid(c.blockAt(x, y, z)) {
			return y
		}
	}

	c.t.Fatalf("no ground at %d,%d", x, z)

	return 0
}

// waitForNearbyChunks waits for the chunks around the client and returns its
// block column.
func (c *testClient) waitForNearbyChunks() (x, z int) {
	c.t.Helper()

	center := world.ChunkPosAt(c.pos[0], c.pos[2])

	waitFor[*packet.ChunkBatchFinished](c, func(*packet.ChunkBatchFinished) bool {
		_, ok := c.chunks[center]
		return ok && len(c.chunks) >= 9
	})

	return int(math.Floor(c.pos[0])), int(math.Floor(c.pos[2]))
}

// placeOnGround places the block item of a hotbar slot on top of the ground
// at x, z and returns where it goes.
func (c *testClient) placeOnGround(slot int16, x, z int, seq int32) packet.BlockPos {
	c.t.Helper()

	ground := packet.BlockPos{X: int32(x), Y: int32(c.groundAt(x, z)), Z: int32(z)}
	c.send(&packet.SetCarriedItem{Slot: slot})
	c.send(&packet.UseItemOn{Pos: ground, Face: int32(block.Up), CursorX: 0.5, CursorY: 1, CursorZ: 0.5, Sequence: seq})

	return packet.BlockPos{X: ground.X, Y: ground.Y + 1, Z: ground.Z}
}

// isBlockUpdate matches the update of the block at pos.
func isBlockUpdate(pos packet.BlockPos) func(*packet.BlockUpdate) bool {
	return func(p *packet.BlockUpdate) bool { return p.Pos == pos }
}

func TestBlockEditing(t *testing.T) {
	dir := t.TempDir()
	srv, addr := startServer(t, testConfig(dir))
	items := registry.MustGet(registry.Item)

	a := dial(t, addr)
	a.login("Alice")
	waitFor[*packet.PlayerPosition](a, nil)

	x, z := a.waitForNearbyChunks()

	b := dial(t, addr)
	b.login("Bob")
	waitFor[*packet.PlayerPosition](b, nil)

	// Alice takes stone, a door and a chest from the creative inventory.
	a.send(&packet.SetCreativeModeSlot{Slot: 36, Stack: item.Stack{Count: 1, Item: items.MustID("minecraft:stone")}})
	a.send(&packet.SetCreativeModeSlot{Slot: 37, Stack: item.Stack{Count: 1, Item: items.MustID("minecraft:oak_door")}})
	a.send(&packet.SetCreativeModeSlot{Slot: 38, Stack: item.Stack{Count: 1, Item: items.MustID("minecraft:chest")}})

	stonePos := a.placeOnGround(0, x+2, z, 1)
	update := waitFor[*packet.BlockUpdate](a, isBlockUpdate(stonePos))

	if block.StateID(update.State) != block.MustParse("stone") {
		t.Fatalf("placed %s, want stone", block.StateID(update.State))
	}

	if ack := waitFor[*packet.BlockChangedAck](a, nil); ack.Sequence != 1 {
		t.Fatalf("acknowledged sequence %d, want 1", ack.Sequence)
	}

	// Other players see the change.
	waitFor[*packet.BlockUpdate](b, isBlockUpdate(stonePos))

	// Both halves of a door are placed.
	doorPos := a.placeOnGround(1, x-2, z, 2)
	lower := waitFor[*packet.BlockUpdate](a, isBlockUpdate(doorPos))
	upper := waitFor[*packet.BlockUpdate](a, isBlockUpdate(packet.BlockPos{X: doorPos.X, Y: doorPos.Y + 1, Z: doorPos.Z}))

	if !strings.Contains(block.StateID(lower.State).String(), "half=lower") || !strings.Contains(block.StateID(upper.State).String(), "half=upper") {
		t.Fatalf("door placed as %s and %s", block.StateID(lower.State), block.StateID(upper.State))
	}

	chestPos := a.placeOnGround(2, x, z+2, 3)
	waitFor[*packet.BlockUpdate](a, isBlockUpdate(chestPos))

	// Breaking the ground in another column.
	brokenPos := packet.BlockPos{X: int32(x), Y: int32(a.groundAt(x, z-2)), Z: int32(z - 2)}
	a.send(&packet.PlayerAction{Action: packet.ActionStartDestroyBlock, Pos: brokenPos, Face: uint8(block.Up), Sequence: 4})

	if broken := waitFor[*packet.BlockUpdate](a, isBlockUpdate(brokenPos)); !block.IsAir(block.StateID(broken.State)) && block.StateID(broken.State) != waterState {
		t.Fatalf("broken block became %s", block.StateID(broken.State))
	}

	waitFor[*packet.BlockChangedAck](a, func(p *packet.BlockChangedAck) bool { return p.Sequence == 4 })

	// Breaking the upper half of the door removes the whole door.
	a.send(&packet.PlayerAction{Action: packet.ActionStartDestroyBlock, Pos: packet.BlockPos{X: doorPos.X, Y: doorPos.Y + 1, Z: doorPos.Z}, Sequence: 5})
	waitFor[*packet.BlockUpdate](a, func(p *packet.BlockUpdate) bool { return p.Pos == doorPos && block.IsAir(block.StateID(p.State)) })

	a.conn.Close()
	b.conn.Close()

	stopWhenEmpty(t, srv)

	// After a restart the changes are still there.
	_, addr2 := startServer(t, testConfig(dir))
	c := dial(t, addr2)
	c.login("Alice")
	waitFor[*packet.ChunkBatchFinished](c, func(*packet.ChunkBatchFinished) bool { return len(c.chunks) >= 9 })

	at := func(p packet.BlockPos) block.StateID { return c.blockAt(int(p.X), int(p.Y), int(p.Z)) }

	if at(stonePos) != block.MustParse("stone") {
		t.Errorf("placed stone is %s after a restart", at(stonePos))
	}

	if s := at(brokenPos); !block.IsAir(s) && s != waterState {
		t.Errorf("broken block is %s after a restart", s)
	}

	if !block.IsAir(at(doorPos)) {
		t.Errorf("broken door is %s after a restart", at(doorPos))
	}

	if block.ByState(at(chestPos)).Name != "minecraft:chest" {
		t.Fatalf("placed chest is %s after a restart", at(chestPos))
	}

	// The chest's block entity is sent with its chunk so the client renders it.
	chunk := c.chunks[world.ChunkPosAt(float64(chestPos.X), float64(chestPos.Z))]
	found := false

	for _, be := range chunk.BlockEntities {
		found = found || (be.Y == int16(chestPos.Y) && be.PackedXZ == uint8(chestPos.X&15)<<4|uint8(chestPos.Z&15) &&
			be.Type == registry.MustGet(registry.BlockEntityType).MustID("minecraft:chest"))
	}

	if !found {
		t.Fatalf("chunk has no chest block entity: %+v", chunk.BlockEntities)
	}
}

func TestInventory(t *testing.T) {
	dir := t.TempDir()
	srv, addr := startServer(t, testConfig(dir))
	items := registry.MustGet(registry.Item)
	stone, log := items.MustID("minecraft:stone"), items.MustID("minecraft:oak_log")

	a := dial(t, addr)
	a.login("Alice")

	if content := waitFor[*packet.ContainerSetContent](a, nil); len(content.Items) != inventory.MenuSize {
		t.Fatalf("inventory sent with %d slots, want %d", len(content.Items), inventory.MenuSize)
	}

	waitFor[*packet.PlayerPosition](a, nil)
	x, z := a.waitForNearbyChunks()

	// Alice places a log and a stone block, then moves the logs to her main
	// inventory.
	a.send(&packet.SetCreativeModeSlot{Slot: 36, Stack: item.Stack{Count: 64, Item: stone}})
	a.send(&packet.SetCreativeModeSlot{Slot: 37, Stack: item.Stack{Count: 5, Item: log}})
	logPos := a.placeOnGround(1, x-2, z, 1)
	stonePos := a.placeOnGround(0, x+2, z, 2)
	waitFor[*packet.BlockUpdate](a, isBlockUpdate(logPos))
	waitFor[*packet.BlockUpdate](a, isBlockUpdate(stonePos))
	a.send(&packet.SetCreativeModeSlot{Slot: 37})
	a.send(&packet.SetCreativeModeSlot{Slot: 20, Stack: item.Stack{Count: 5, Item: log}})

	pick := func(pos packet.BlockPos, held int32, changed ...packet.ContainerSetSlot) {
		t.Helper()
		a.send(&packet.PickItemFromBlock{Pos: pos})

		if got := waitFor[*packet.SetHeldSlot](a, nil); got.Slot != held {
			t.Fatalf("picking selected hotbar slot %d, want %d", got.Slot, held)
		}

		for _, want := range changed {
			if got := waitFor[*packet.ContainerSetSlot](a, nil); !reflect.DeepEqual(*got, want) {
				t.Fatalf("picking changed %+v, want %+v", *got, want)
			}
		}
	}

	// Stone in the hotbar is selected.
	a.send(&packet.SetCarriedItem{Slot: 4})
	pick(stonePos, 0)

	// The logs are swapped into the first empty hotbar slot from the selected one.
	pick(logPos, 1,
		packet.ContainerSetSlot{Slot: 37, Stack: item.Stack{Count: 5, Item: log}},
		packet.ContainerSetSlot{Slot: 20})

	// Alice drops one log, then all of her stone.
	a.send(&packet.PlayerAction{Action: packet.ActionDropItem})
	a.send(&packet.SetCarriedItem{Slot: 0})
	a.send(&packet.PlayerAction{Action: packet.ActionDropAllItems})

	// A creative player picking stone it no longer has gets one.
	pick(stonePos, 0, packet.ContainerSetSlot{Slot: 36, Stack: item.Stack{Count: 1, Item: stone}})

	a.send(&packet.SetCarriedItem{Slot: 1})
	a.conn.Close()

	stopWhenEmpty(t, srv)

	// After a restart Alice gets her inventory and hotbar slot back.
	_, addr2 := startServer(t, testConfig(dir))
	b := dial(t, addr2)
	b.login("Alice")

	if held := waitFor[*packet.SetHeldSlot](b, nil); held.Slot != 1 {
		t.Errorf("hotbar slot %d restored, want 1", held.Slot)
	}

	want := make([]item.Stack, inventory.MenuSize)
	want[36] = item.Stack{Count: 1, Item: stone}
	want[37] = item.Stack{Count: 4, Item: log}

	if content := waitFor[*packet.ContainerSetContent](b, nil); !reflect.DeepEqual(content.Items, want) {
		t.Fatalf("inventory restored as %+v, want %+v", content.Items, want)
	}
}

// Every item a block can be picked as must exist.
func TestPickItemsExist(t *testing.T) {
	for s := block.StateID(0); int(s) < block.StateCount(); s++ {
		if name, ok := block.PickItem(s); ok {
			if _, found := itemRegistry.ID(name); !found {
				t.Fatalf("%s is picked as unknown item %s", s, name)
			}
		}
	}
}

// A handler's kick error disconnects the player with its reason.
func TestInvalidMovementKick(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))
	a := dial(t, addr)
	a.login("Alice")
	waitFor[*packet.PlayerPosition](a, nil)

	a.send(&packet.MovePlayerPos{X: math.NaN(), Y: 70, Z: 0})

	if kick := waitFor[*packet.Disconnect](a, nil); kick.Reason.Translate != "multiplayer.disconnect.invalid_player_movement" {
		t.Fatalf("disconnected with %+v", kick.Reason)
	}
}

func TestArmSwingAndSwapHands(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))
	stone := registry.MustGet(registry.Item).MustID("minecraft:stone")

	a := dial(t, addr)
	alice := a.login("Alice").EntityID
	waitFor[*packet.PlayerPosition](a, nil)
	x, z := a.waitForNearbyChunks()

	b := dial(t, addr)
	b.login("Bob")
	waitFor[*packet.AddEntity](b, func(p *packet.AddEntity) bool { return p.EntityID == alice })

	isAliceSwing := func(p *packet.SwingAnimation) bool { return p.EntityID == alice }

	// Bob sees Alice swing when she places a block and when she punches.
	a.send(&packet.SetCreativeModeSlot{Slot: 36, Stack: item.Of(stone, 64)})
	a.placeOnGround(0, x+2, z, 1)

	if swing := waitFor[*packet.SwingAnimation](b, isAliceSwing); swing.Hand != packet.HandMain || swing.Animation != item.DefaultSwing {
		t.Fatalf("placing showed %+v", swing)
	}

	// A new swing starts once the last one is past halfway.
	time.Sleep(4 * tickDuration)
	a.send(&packet.Punch{})
	waitFor[*packet.SwingAnimation](b, isAliceSwing)

	// The swap key moves the stone to Alice's off hand.
	a.send(&packet.PlayerAction{Action: packet.ActionSwapWithOffHand})

	for _, want := range []packet.ContainerSetSlot{{Slot: 36}, {Slot: 45, Stack: item.Of(stone, 64)}} {
		if got := waitFor[*packet.ContainerSetSlot](a, nil); !reflect.DeepEqual(*got, want) {
			t.Fatalf("swapping hands sent %+v, want %+v", *got, want)
		}
	}
}

// The world's icon shows in the server list and is sent on join, and the
// configured links reach the client during configuration.
func TestServerIconAndLinks(t *testing.T) {
	dir := t.TempDir()
	icon := writePNG(t, filepath.Join(dir, "icon.png"), faviconSize)
	conf := testConfig(dir)
	conf.ServerLinks = []config.ServerLink{{Label: "website", URL: "https://example.com"}}
	_, addr := startServer(t, conf)

	c := dial(t, addr)
	c.handshake(packet.IntentStatus, version.Protocol)
	c.send(&packet.StatusRequest{})

	var status struct {
		Favicon string `json:"favicon"`
	}

	if err := json.Unmarshal([]byte(waitFor[*packet.StatusResponse](c, nil).JSON), &status); err != nil {
		t.Fatal(err)
	}

	if want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(icon); status.Favicon != want {
		t.Fatalf("status favicon is %.40q…", status.Favicon)
	}

	a := dial(t, addr)
	a.login("Alice")

	if want := []packet.ServerLink{{Type: packet.LinkWebsite, URL: "https://example.com"}}; a.links == nil || !reflect.DeepEqual(a.links.Links, want) {
		t.Fatalf("configuration sent links %+v", a.links)
	}

	if data := waitFor[*packet.ServerData](a, nil); data.Motd.Text != "Test server" || !bytes.Equal(data.Icon, icon) {
		t.Fatalf("server data was %q with a %d byte icon", data.Motd.Text, len(data.Icon))
	}
}

func TestListCommand(t *testing.T) {
	_, addr := startServer(t, testConfig(t.TempDir()))

	a := dial(t, addr)
	a.login("Alice")
	waitFor[*packet.PlayerPosition](a, nil)

	b := dial(t, addr)
	bob := b.login("Bob").EntityID
	waitFor[*packet.AddEntity](a, func(p *packet.AddEntity) bool { return p.EntityID == bob })

	if _, ok := rootCommand(waitFor[*packet.Commands](b, nil), "list"); !ok {
		t.Fatal("non-operators must see /list")
	}

	isList := func(p *packet.SystemChat) bool { return p.Content.Translate == "commands.list.players" }

	// Anyone may list the players, in the order they joined.
	a.send(&packet.ChatCommand{Command: "list"})
	list := waitFor[*packet.SystemChat](a, isList).Content

	if len(list.With) != 3 || list.With[0].Text != "2" || list.With[1].Text != "10" || list.With[2].PlainText() != "Alice, Bob" {
		t.Fatalf("/list replied %+v", list)
	}

	a.send(&packet.ChatCommand{Command: "list uuids"})
	names := waitFor[*packet.SystemChat](a, isList).Content.With[2].Extra

	if len(names) != 3 || names[0].Translate != "commands.list.nameAndId" || names[0].With[1].Text != uuid.OfflinePlayer("Alice").String() {
		t.Fatalf("/list uuids replied %+v", names)
	}
}
