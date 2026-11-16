package storage

import (
	"path/filepath"

	"main/src/protocol"
	"main/src/uuid"
	"main/src/version"
)

// Player files hold: magic "GMCP", version u8, x, y, z f64, yaw, pitch f32.
// Version 2 adds the held hotbar slot u8, the protocol number the item
// components were encoded with, and a VarInt count of inventory items, each
// written by writeItem.
const (
	playerMagic   = "GMCP"
	playerVersion = 2

	// maxInventoryItems bounds the item count read from a player file.
	maxInventoryItems = 64
)

// PlayerData is the persisted state of a player.
type PlayerData struct {
	X, Y, Z    float64
	Yaw, Pitch float32
	HeldSlot   int
	Inventory  []InventoryItem
}

// InventoryItem is a stack in a player's inventory. Items are stored by name
// so they survive registry renumbering. Component values embed protocol IDs,
// so they are dropped when a save is loaded by a different protocol version.
type InventoryItem struct {
	Slot       int
	Item       string
	Count      int32
	Components []ItemComponent
	Removed    []int32
}

// ItemComponent is an encoded data component of an inventory item.
type ItemComponent struct {
	Type int32
	Data []byte
}

func (s *Store) playerPath(id uuid.UUID) string {
	return filepath.Join(s.dir, "players", id.String()+".dat")
}

// LoadPlayer returns the saved state of a player and whether one exists.
func (s *Store) LoadPlayer(id uuid.UUID) (PlayerData, bool, error) {
	r, fileVersion, found, err := readVersioned(s.playerPath(id), playerMagic, playerVersion)

	if !found || err != nil {
		return PlayerData{}, false, err
	}

	p := PlayerData{X: r.Double(), Y: r.Double(), Z: r.Double(), Yaw: r.Float(), Pitch: r.Float()}

	if fileVersion >= 2 {
		p.HeldSlot = int(r.UByte())
		sameProtocol := r.VarInt() == version.Protocol

		p.Inventory = protocol.ReadList(r, maxInventoryItems, func() InventoryItem {
			item := readItem(r)

			if !sameProtocol {
				item.Components, item.Removed = nil, nil
			}

			return item
		})
	}

	if err = r.Finish(); err != nil {
		return PlayerData{}, false, err
	}

	return p, true, nil
}

// SavePlayer stores the state of a player.
func (s *Store) SavePlayer(id uuid.UUID, p PlayerData) error {
	return writeVersioned(s.playerPath(id), playerMagic, playerVersion, func(w *protocol.Writer) {
		w.Double(p.X)
		w.Double(p.Y)
		w.Double(p.Z)
		w.Float(p.Yaw)
		w.Float(p.Pitch)
		w.UByte(uint8(p.HeldSlot))
		w.VarInt(version.Protocol)
		protocol.WriteList(w, p.Inventory, func(item InventoryItem) { writeItem(w, item) })
	})
}

// writeItem writes slot u8, item name, count, a VarInt count of components
// that each have a VarInt type and a length-prefixed value, and a VarInt count
// of removed component types.
func writeItem(w *protocol.Writer, item InventoryItem) {
	w.UByte(uint8(item.Slot))
	w.String(item.Item)
	w.VarInt(item.Count)
	protocol.WriteList(w, item.Components, func(c ItemComponent) {
		w.VarInt(c.Type)
		w.ByteArray(c.Data)
	})
	protocol.WriteList(w, item.Removed, w.VarInt)
}

func readItem(r *protocol.Reader) InventoryItem {
	item := InventoryItem{Slot: int(r.UByte()), Item: r.String(256), Count: r.VarInt()}
	item.Components = protocol.ReadList(r, r.Remaining(), func() ItemComponent {
		return ItemComponent{Type: r.VarInt(), Data: r.ByteArray(r.Remaining())}
	})
	item.Removed = protocol.ReadList(r, r.Remaining(), r.VarInt)

	return item
}
