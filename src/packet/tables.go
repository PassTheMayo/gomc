package packet

// Packet IDs are the registration order of each protocol in the vanilla
// server's HandshakeProtocols, StatusProtocols, LoginProtocols,
// ConfigurationProtocols and GameProtocols classes. Packets the server neither
// sends nor handles are left out; unknown serverbound packets are ignored.
var (
	HandshakeServerbound = newTable("handshake/serverbound", map[int32]Packet{
		0x00: (*Intention)(nil),
	})

	StatusServerbound = newTable("status/serverbound", map[int32]Packet{
		0x00: (*StatusRequest)(nil),
		0x01: (*PingRequest)(nil),
	})

	StatusClientbound = newTable("status/clientbound", map[int32]Packet{
		0x00: (*StatusResponse)(nil),
		0x01: (*PongResponse)(nil),
	})

	LoginServerbound = newTable("login/serverbound", map[int32]Packet{
		0x00: (*LoginHello)(nil),
		0x01: (*EncryptionResponse)(nil),
		0x03: (*LoginAcknowledged)(nil),
	})

	LoginClientbound = newTable("login/clientbound", map[int32]Packet{
		0x00: (*LoginDisconnect)(nil),
		0x01: (*EncryptionRequest)(nil),
		0x02: (*LoginFinished)(nil),
		0x03: (*LoginCompression)(nil),
	})

	ConfigurationServerbound = newTable("configuration/serverbound", map[int32]Packet{
		0x00: (*ClientInformation)(nil),
		0x02: (*CustomPayload)(nil),
		0x03: (*FinishConfiguration)(nil),
		0x04: (*KeepAlive)(nil),
		0x07: (*SelectKnownPacks)(nil),
	})

	ConfigurationClientbound = newTable("configuration/clientbound", map[int32]Packet{
		0x01: (*CustomPayload)(nil),
		0x02: (*Disconnect)(nil),
		0x03: (*FinishConfiguration)(nil),
		0x04: (*KeepAlive)(nil),
		0x07: (*RegistryData)(nil),
		0x0D: (*UpdateEnabledFeatures)(nil),
		0x0E: (*UpdateTags)(nil),
		0x0F: (*SelectKnownPacks)(nil),
		0x11: (*ServerLinks)(nil),
	})

	PlayServerbound = newTable("play/serverbound", map[int32]Packet{
		0x00: (*AcceptTeleportation)(nil),
		0x07: (*ChatCommand)(nil),
		0x0B: (*ChunkBatchReceived)(nil),
		0x0E: (*ClientInformation)(nil),
		0x1C: (*KeepAlive)(nil),
		0x1E: (*MovePlayerPos)(nil),
		0x1F: (*MovePlayerPosRot)(nil),
		0x20: (*MovePlayerRot)(nil),
		0x21: (*MovePlayerStatusOnly)(nil),
		0x24: (*PickItemFromBlock)(nil),
		0x28: (*PlayerAbilities)(nil),
		0x29: (*PlayerAction)(nil),
		0x2A: (*PlayerCommand)(nil),
		0x2B: (*PlayerInput)(nil),
		0x2C: (*PlayerLoaded)(nil),
		0x2E: (*Punch)(nil),
		0x36: (*SetCarriedItem)(nil),
		0x39: (*SetCreativeModeSlot)(nil),
		0x42: (*UseItemOn)(nil),
	})

	PlayClientbound = newTable("play/clientbound", map[int32]Packet{
		0x01: (*AddEntity)(nil),
		0x04: (*BlockChangedAck)(nil),
		0x08: (*BlockUpdate)(nil),
		0x0B: (*ChunkBatchFinished)(nil),
		0x0C: (*ChunkBatchStart)(nil),
		0x10: (*Commands)(nil),
		0x12: (*ContainerSetContent)(nil),
		0x14: (*ContainerSetSlot)(nil),
		0x20: (*Disconnect)(nil),
		0x23: (*EntityPositionSync)(nil),
		0x26: (*ForgetLevelChunk)(nil),
		0x27: (*GameEvent)(nil),
		0x2D: (*KeepAlive)(nil),
		0x2E: (*LevelChunkWithLight)(nil),
		0x32: (*Login)(nil),
		0x36: (*MoveEntityPos)(nil),
		0x37: (*MoveEntityPosRot)(nil),
		0x39: (*MoveEntityRot)(nil),
		0x41: (*PlayerAbilitiesUpdate)(nil),
		0x46: (*PlayerInfoRemove)(nil),
		0x47: (*PlayerInfoUpdate)(nil),
		0x49: (*PlayerPosition)(nil),
		0x4E: (*RemoveEntities)(nil),
		0x55: (*RotateHead)(nil),
		0x58: (*ServerData)(nil),
		0x60: (*SetChunkCacheCenter)(nil),
		0x63: (*SetDefaultSpawnPosition)(nil),
		0x65: (*SetEntityData)(nil),
		0x6B: (*SetHeldSlot)(nil),
		0x73: (*SetTime)(nil),
		0x7B: (*SwingAnimation)(nil),
		0x7C: (*SystemChat)(nil),
	})
)
