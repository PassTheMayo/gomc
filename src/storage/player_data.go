package storage

import (
	"errors"
	"os"
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
	data, err := os.ReadFile(s.playerPath(id))

	if errors.Is(err, os.ErrNotExist) {
		return PlayerData{}, false, nil
	}

	if err != nil {
		return PlayerData{}, false, err
	}

	r := protocol.NewReader(data)
	magic, fileVersion := string(r.Raw(4)), r.UByte()

	if magic != playerMagic || fileVersion < 1 || fileVersion > playerVersion {
		return PlayerData{}, false, errors.New("storage: unsupported player file")
	}

	p := PlayerData{X: r.Double(), Y: r.Double(), Z: r.Double(), Yaw: r.Float(), Pitch: r.Float()}

	if fileVersion >= 2 {
		p.HeldSlot = int(r.UByte())
		sameProtocol := r.VarInt() == version.Protocol

		for n := r.Length(maxInventoryItems); n > 0 && r.Err() == nil; n-- {
			item := readItem(r)

			if !sameProtocol {
				item.Components, item.Removed = nil, nil
			}

			p.Inventory = append(p.Inventory, item)
		}
	}

	if err = r.Finish(); err != nil {
		return PlayerData{}, false, err
	}

	return p, true, nil
}

// SavePlayer stores the state of a player.
func (s *Store) SavePlayer(id uuid.UUID, p PlayerData) error {
	w := protocol.NewWriter()
	w.Raw([]byte(playerMagic))
	w.UByte(playerVersion)
	w.Double(p.X)
	w.Double(p.Y)
	w.Double(p.Z)
	w.Float(p.Yaw)
	w.Float(p.Pitch)
	w.UByte(uint8(p.HeldSlot))
	w.VarInt(version.Protocol)
	w.VarInt(int32(len(p.Inventory)))

	for _, item := range p.Inventory {
		writeItem(w, item)
	}

	return writeFileAtomic(s.playerPath(id), w.Bytes())
}

// writeItem writes slot u8, item name, count, a VarInt count of components
// that each have a VarInt type and a length-prefixed value, and a VarInt count
// of removed component types.
func writeItem(w *protocol.Writer, item InventoryItem) {
	w.UByte(uint8(item.Slot))
	w.String(item.Item)
	w.VarInt(item.Count)
	w.VarInt(int32(len(item.Components)))

	for _, c := range item.Components {
		w.VarInt(c.Type)
		w.ByteArray(c.Data)
	}

	w.VarInt(int32(len(item.Removed)))

	for _, t := range item.Removed {
		w.VarInt(t)
	}
}

func readItem(r *protocol.Reader) InventoryItem {
	item := InventoryItem{Slot: int(r.UByte()), Item: r.String(256), Count: r.VarInt()}

	for n := r.Length(r.Remaining()); n > 0 && r.Err() == nil; n-- {
		item.Components = append(item.Components, ItemComponent{Type: r.VarInt(), Data: r.ByteArray(r.Remaining())})
	}

	for n := r.Length(r.Remaining()); n > 0 && r.Err() == nil; n-- {
		item.Removed = append(item.Removed, r.VarInt())
	}

	return item
}
