package world

import (
	"main/src/block"
	"main/src/packet"
	"main/src/protocol"
	"main/src/registry"
)

var blockEntityTypes = registry.MustGet(registry.BlockEntityType)

func hasBlockEntity(v int32) bool {
	_, ok := block.EntityType(block.StateID(v))
	return ok
}

const lightArraySize = 2048

// fullSkyLight is shared by every section that is fully exposed to the sky.
var fullSkyLight = func() []byte {
	b := make([]byte, lightArraySize)

	for i := range b {
		b[i] = 0xFF
	}

	return b
}()

func (c *Chunk) buildPacket() *packet.LevelChunkWithLight {
	w := protocol.NewWriter()

	for _, s := range c.sections {
		s.Encode(w)
	}

	light, heightmaps := c.scanColumns()

	return &packet.LevelChunkWithLight{
		X:             c.Pos.X,
		Z:             c.Pos.Z,
		Heightmaps:    heightmaps,
		Data:          w.Bytes(),
		BlockEntities: c.blockEntities(),
		Light:         light,
	}
}

// blockEntities lists the block entities the chunk's blocks need, such as
// chests and signs. They carry no data: the client creates them with their
// default contents, which is all the server knows about them.
func (c *Chunk) blockEntities() []packet.BlockEntity {
	var out []packet.BlockEntity

	for si, sec := range c.sections {
		if sec.IsEmpty() || !sec.blocks.MayContain(hasBlockEntity) {
			continue
		}

		for i, v := range sec.BlockValues() {
			if typ, ok := block.EntityType(block.StateID(v)); ok {
				out = append(out, packet.BlockEntity{
					PackedXZ: uint8(i&15)<<4 | uint8(i>>4&15),
					Y:        int16(c.dim.MinY + si*16 + i>>8),
					Type:     blockEntityTypes.MustID(typ),
				})
			}
		}
	}

	return out
}

// scanColumns computes sky light and heightmaps in one top-down pass over
// every column. Sky light falls straight down and is dimmed by translucent
// blocks; it does not spread sideways, which is accurate for open terrain.
// Block light is always empty because no generated block emits light.
func (c *Chunk) scanColumns() (packet.LightData, []packet.Heightmap) {
	n := len(c.sections)
	sky := make([][lightArraySize]byte, n)
	blocks := make([][]int32, n)
	var surface, motion [256]int

	for si, sec := range c.sections {
		if !sec.IsEmpty() {
			blocks[si] = sec.BlockValues()
		}
	}

	for z := range 16 {
		for x := range 16 {
			col := z<<4 | x
			level := uint8(15)

			for si := n - 1; si >= 0; si-- {
				for ly := 15; ly >= 0; ly-- {
					if blocks[si] != nil {
						s := block.StateID(blocks[si][ly<<8|col])
						height := si*16 + ly + 1

						if surface[col] == 0 && !block.IsAir(s) {
							surface[col] = height
						}

						if motion[col] == 0 && block.BlocksMotion(s) {
							motion[col] = height
						}

						if op := block.LightOpacity(s); op >= level {
							level = 0
						} else {
							level -= op
						}
					}

					setNibble(&sky[si], ly<<8|col, level)
				}
			}
		}
	}

	var light packet.LightData

	// Light sections start one below the world and end one above it.
	light.EmptySkyMask.Set(0)

	for si := range n {
		switch classify(&sky[si]) {
		case 0:
			light.EmptySkyMask.Set(si + 1)
		case 15:
			light.SkyMask.Set(si + 1)
			light.SkyLight = append(light.SkyLight, fullSkyLight)
		default:
			light.SkyMask.Set(si + 1)
			light.SkyLight = append(light.SkyLight, append([]byte(nil), sky[si][:]...))
		}
	}

	light.SkyMask.Set(n + 1)
	light.SkyLight = append(light.SkyLight, fullSkyLight)

	for i := range n + 2 {
		light.EmptyBlockMask.Set(i)
	}

	heightmaps := []packet.Heightmap{
		{Type: packet.HeightmapWorldSurface, Data: packHeightmap(&surface, c.dim.Height)},
		{Type: packet.HeightmapMotionBlocking, Data: packHeightmap(&motion, c.dim.Height)},
	}

	return light, heightmaps
}

func setNibble(arr *[lightArraySize]byte, index int, level uint8) {
	if index&1 == 0 {
		arr[index>>1] = arr[index>>1]&0xF0 | level
	} else {
		arr[index>>1] = arr[index>>1]&0x0F | level<<4
	}
}

// classify returns 0 or 15 if every level in arr has that value, otherwise -1.
func classify(arr *[lightArraySize]byte) int {
	first := arr[0]

	if first != 0 && first != 0xFF {
		return -1
	}

	for _, b := range arr {
		if b != first {
			return -1
		}
	}

	return int(first & 15)
}

// packHeightmap packs 256 heights into longs without letting entries span two longs.
func packHeightmap(heights *[256]int, worldHeight int) []uint64 {
	bits := bitsFor(worldHeight + 1)
	perLong := 64 / bits
	data := make([]uint64, (256+perLong-1)/perLong)

	for i, h := range heights {
		data[i/perLong] |= uint64(h) << ((i % perLong) * bits)
	}

	return data
}
