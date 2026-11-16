package packet

import (
	"math"
	"reflect"
	"testing"

	"main/src/item"
	"main/src/nbt"
	"main/src/profile"
	"main/src/protocol"
	"main/src/registry"
	"main/src/text"
	"main/src/uuid"
)

func TestRoundTrip(t *testing.T) {
	id := uuid.New()
	prof := profile.GameProfile{ID: id, Name: "Steve", Properties: []profile.Property{
		{Name: "textures", Value: "abc", Signature: "sig"},
		{Name: "other", Value: "def"},
	}}
	name := text.Text("Display").Colored(text.Red)

	var mask protocol.BitSet
	mask.Set(1)
	mask.Set(25)

	// Angles are multiples of 360/256 degrees so they survive byte quantization.
	cases := map[*Table][]Packet{
		HandshakeServerbound: {
			&Intention{ProtocolVersion: 777, ServerAddress: "localhost", ServerPort: 25565, Intent: IntentLogin},
		},
		StatusServerbound: {&StatusRequest{}, &PingRequest{Payload: 42}},
		StatusClientbound: {&StatusResponse{JSON: `{"a":1}`}, &PongResponse{Payload: -42}},
		LoginServerbound: {
			&LoginHello{Name: "Steve", UUID: id},
			&EncryptionResponse{SharedSecret: []byte{1, 2, 3}, Challenge: []byte{4, 5}},
			&LoginAcknowledged{},
		},
		LoginClientbound: {
			&LoginDisconnect{Reason: text.Translatable("key", text.Text("arg"))},
			&EncryptionRequest{PublicKey: []byte{9, 9}, Challenge: []byte{1, 2, 3, 4}, ShouldAuthenticate: true},
			&LoginFinished{Profile: prof, SessionID: uuid.New()},
			&LoginCompression{Threshold: 256},
		},
		ConfigurationServerbound: {
			&ClientInformation{Locale: "en_us", ViewDistance: 12, ChatColors: true, ModelCustomisation: 0x7F, MainHand: 1, AllowsListing: true},
			&CustomPayload{Channel: "minecraft:brand", Data: []byte{5, 'v', 'a', 'n', 'i', 'l'}},
			&FinishConfiguration{},
			&KeepAlive{ID: 7},
			&SelectKnownPacks{Packs: []KnownPack{{Namespace: "minecraft", ID: "core", Version: "26.3"}}},
		},
		ConfigurationClientbound: {
			&Disconnect{Reason: text.Text("bye").Colored(text.Red)},
			&RegistryData{Registry: "minecraft:dimension_type", Entries: []RegistryEntry{
				{ID: "minecraft:overworld"},
				{ID: "minecraft:custom", Data: nbt.Compound{{Name: "height", Value: nbt.Int(384)}}},
			}},
			&ServerLinks{Links: []ServerLink{
				{Type: LinkWebsite, URL: "https://example.com"},
				{Type: LinkCustom, Label: text.Text("Map"), URL: "https://example.com/map"},
			}},
			&UpdateEnabledFeatures{Features: []string{"minecraft:vanilla"}},
			&UpdateTags{Tags: map[string][]registry.Tag{"minecraft:fluid": {{Name: "minecraft:water", Entries: []int32{1, 2}}}}},
		},
		PlayServerbound: {
			&AcceptTeleportation{TeleportID: 3, X: 1.5, Y: 64, Z: -2.5, Yaw: 90, Pitch: -45},
			&ChatCommand{Command: "pregen 10"},
			&ChunkBatchReceived{DesiredChunksPerTick: 12.5},
			&MovePlayerPos{X: 1, Y: 2, Z: 3, MoveFlags: MoveFlags{OnGround: true}},
			&MovePlayerPosRot{X: 1, Y: 2, Z: 3, Yaw: 10, Pitch: 20, MoveFlags: MoveFlags{HorizontalCollision: true}},
			&MovePlayerRot{Yaw: 10, Pitch: 20, MoveFlags: MoveFlags{OnGround: true, HorizontalCollision: true}},
			&MovePlayerStatusOnly{MoveFlags: MoveFlags{OnGround: true}},
			&PlayerAbilities{Flags: AbilityFlying},
			&PlayerCommand{EntityID: 5, Action: CommandStartSprinting},
			&PlayerInput{Flags: InputSneak | 1},
			&PlayerLoaded{},
			&Punch{},
			&PlayerAction{Action: ActionStopDestroyBlock, Pos: BlockPos{X: -5, Y: -60, Z: 300}, Face: 1, Sequence: 12},
			&UseItemOn{Hand: HandOff, Pos: BlockPos{X: 1, Y: 2, Z: 3}, Face: 4, CursorX: 0.5, CursorY: 1, CursorZ: 0.25, Inside: true, Sequence: 99},
			&SetCarriedItem{Slot: 8},
			&SetCreativeModeSlot{Slot: 36, Stack: item.Stack{Count: 64, Item: 1, Components: []item.Component{{Type: 5, Data: []byte{1, 2}}}, Removed: []int32{7}}},
			&SetCreativeModeSlot{Slot: 45},
			&PickItemFromBlock{Pos: BlockPos{X: -7, Y: 80, Z: 12}, IncludeData: true},
		},
		PlayClientbound: {
			&AddEntity{EntityID: 9, UUID: id, Type: 159, X: 1.5, Y: 70, Z: -3.25, Pitch: -45, Yaw: 90, HeadYaw: 45},
			&ChunkBatchStart{},
			&BlockChangedAck{Sequence: 12},
			&BlockUpdate{Pos: BlockPos{X: 1, Y: -64, Z: -1}, State: 9},
			&ChunkBatchFinished{BatchSize: 9},
			&ContainerSetContent{StateID: 3, Items: []item.Stack{{}, {Count: 64, Item: 1, Removed: []int32{4}}, {}}},
			&ContainerSetSlot{ContainerID: InventoryContainer, Slot: 36, Stack: item.Stack{Count: 1, Item: 2}},
			&Commands{Root: 0, Nodes: []CommandNode{
				{Type: NodeRoot, Children: []int32{1}},
				{Type: NodeLiteral, Name: "pregen", Restricted: true, Children: []int32{2, 3}},
				{Type: NodeArgument, Name: "radius", Executable: true, Parser: ParserInteger, Properties: IntegerProperties(1, math.MaxInt32)},
				{Type: NodeArgument, Name: "any", Executable: true, Parser: ParserInteger, Properties: IntegerProperties(math.MinInt32, 5)},
			}},
			&Commands{Root: 0, Nodes: []CommandNode{
				{Type: NodeRoot, Children: []int32{1}},
				{Type: NodeLiteral, Name: "gamemode", Restricted: true, Children: []int32{2}},
				{Type: NodeArgument, Name: "gamemode", Executable: true, Parser: ParserGameMode, Children: []int32{3}},
				{Type: NodeArgument, Name: "target", Executable: true, Parser: ParserEntity, Properties: []byte{EntityPlayersOnly}},
			}},
			&EntityPositionSync{EntityID: 9, X: 1, Y: 2, Z: 3, Yaw: 33.3, Pitch: -12, OnGround: true},
			&ForgetLevelChunk{X: -5, Z: 12},
			&GameEvent{Event: GameEventLevelChunksLoadStart},
			&GameEvent{Event: GameEventChangeGameMode, Value: float32(GameModeSpectator)},
			&LevelChunkWithLight{
				X: 3, Z: -4,
				Heightmaps: []Heightmap{{Type: HeightmapMotionBlocking, Data: []uint64{1, 2, 3}}},
				Data:       []byte{0, 1, 2, 3},
				BlockEntities: []BlockEntity{
					{PackedXZ: 0x12, Y: 64, Type: 7},
					{PackedXZ: 0x34, Y: -10, Type: 8, Data: nbt.Append(nil, nbt.Compound{{Name: "id", Value: nbt.String("x")}})},
				},
				Light: LightData{SkyMask: mask, EmptyBlockMask: mask, SkyLight: [][]byte{make([]byte, 2048), make([]byte, 2048)}},
			},
			&Login{
				EntityID: 1, Dimensions: []string{"minecraft:overworld"}, MaxPlayers: 20, ViewDistance: 10, SimulationDistance: 8,
				ShowDeathScreen: true, OnlineMode: true,
				Spawn: SpawnInfo{
					DimensionType: 0, Dimension: "minecraft:overworld", HashedSeed: -99, GameMode: GameModeCreative, PreviousGameMode: -1,
					DeathLocation: &GlobalPos{Dimension: "minecraft:overworld", X: 1, Y: -2, Z: 3}, SeaLevel: 63,
				},
			},
			&MoveEntityPos{EntityID: 9, DX: -4096, DY: 1, DZ: 32767, OnGround: true},
			&MoveEntityPosRot{EntityID: 9, DX: 1, DY: -1, DZ: 0, Yaw: 45, Pitch: 90},
			&MoveEntityRot{EntityID: 9, Yaw: -90, Pitch: 45, OnGround: true},
			&PlayerAbilitiesUpdate{Flags: AbilityInvulnerable | AbilityCanFly | AbilityInstabuild, FlyingSpeed: DefaultFlyingSpeed, WalkingSpeed: DefaultWalkingSpeed},
			&PlayerInfoRemove{UUIDs: []uuid.UUID{id}},
			&PlayerInfoUpdate{Actions: 0xFF &^ InfoInitializeChat, Entries: []PlayerInfoEntry{
				{Profile: prof, GameMode: 1, Listed: true, Latency: 50, DisplayName: &name, ListOrder: 2, ShowHat: true},
			}},
			&PlayerPosition{TeleportID: 1, X: 1, Y: 2, Z: 3, VX: 0.5, Yaw: 180, Pitch: 0, Relative: 1 << 3},
			&RemoveEntities{EntityIDs: []int32{1, 2, 3}},
			&RotateHead{EntityID: 9, HeadYaw: 90},
			&ServerData{Motd: text.Text("Hello"), Icon: []byte{0x89, 'P', 'N', 'G'}},
			&ServerData{Motd: text.Text("No icon")},
			&SetChunkCacheCenter{X: -1, Z: 7},
			&SetDefaultSpawnPosition{Spawn: GlobalPos{Dimension: "minecraft:overworld", X: 0, Y: 70, Z: 0}, Yaw: 90},
			&SetEntityData{EntityID: 9, Values: []EntityDataValue{
				{Index: DataSharedFlags, Type: DataTypeByte, Byte: FlagCrouching},
				{Index: DataPose, Type: DataTypePose, Pose: PoseCrouching},
			}},
			&SetHeldSlot{Slot: 4},
			&SwingAnimation{EntityID: 9, Hand: HandOff, Animation: item.SwingAnimation{Type: item.SwingStab, Duration: 19}},
			&SetTime{GameTime: 1000, Clocks: []ClockState{{Clock: 0, TotalTicks: 24000, Rate: 1}}},
			&SystemChat{Content: text.Translatable("multiplayer.player.joined", text.Text("Steve")).Colored(text.Yellow)},
		},
	}

	for table, packets := range cases {
		for _, p := range packets {
			decoded, err := table.Decode(table.ID(p), Encode(p))

			if err != nil {
				t.Errorf("%T: %v", p, err)
				continue
			}

			if !reflect.DeepEqual(decoded, p) {
				t.Errorf("%T round trip mismatch:\n got %#v\nwant %#v", p, decoded, p)
			}
		}
	}
}

// Clients length-prefix each component value; the server sends them bare.
func TestItemStackForms(t *testing.T) {
	stack := item.Stack{Count: 2, Item: 300, Components: []item.Component{{Type: 13, Data: []byte{1, 9}}}, Removed: []int32{5}}

	server := protocol.NewWriter()
	writeStack(server, stack)

	if want := []byte{2, 0xAC, 0x02, 1, 1, 13, 1, 9, 5}; !reflect.DeepEqual(server.Bytes(), want) {
		t.Errorf("server-sent stack encoded as %v, want %v", server.Bytes(), want)
	}

	client := protocol.NewWriter()
	writeDelimitedStack(client, stack)

	if want := []byte{2, 0xAC, 0x02, 1, 1, 13, 2, 1, 9, 5}; !reflect.DeepEqual(client.Bytes(), want) {
		t.Errorf("client-sent stack encoded as %v, want %v", client.Bytes(), want)
	}

	r := protocol.NewReader(server.Bytes())
	readStack(r)

	if r.Err() == nil {
		t.Error("decoding a server-sent stack with components succeeded")
	}
}

func TestUnknownPacket(t *testing.T) {
	if _, err := PlayServerbound.Decode(0x7F, nil); err == nil {
		t.Fatal("expected an error for an unknown packet")
	}
}
