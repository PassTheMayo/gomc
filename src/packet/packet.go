// Package packet defines the packets of the supported protocol version and the
// tables that assign them IDs in each connection state.
package packet

import (
	"errors"
	"fmt"
	"reflect"

	"main/src/nbt"
	"main/src/protocol"
	"main/src/text"
)

// Packet is the payload of a packet, encodable and decodable in both directions.
type Packet interface {
	Encode(w *protocol.Writer)
	Decode(r *protocol.Reader)
}

var (
	// ErrUnknownPacket is returned when decoding an ID that is not in a table.
	ErrUnknownPacket = errors.New("packet: unknown packet id")

	errSteppedMove         = errors.New("packet: stepped entity moves are not supported")
	errUnsupportedDataType = errors.New("packet: unsupported entity data type")
)

// Table maps packet IDs to packet types for one connection state and direction.
type Table struct {
	name  string
	ids   map[reflect.Type]int32
	types map[int32]reflect.Type
}

func newTable(name string, packets map[int32]Packet) *Table {
	t := &Table{name: name, ids: make(map[reflect.Type]int32), types: make(map[int32]reflect.Type)}

	for id, p := range packets {
		typ := reflect.TypeOf(p)

		if _, dup := t.ids[typ]; dup {
			panic(fmt.Sprintf("packet: %s registered twice in %s", typ, name))
		}

		t.ids[typ] = id
		t.types[id] = typ.Elem()
	}

	return t
}

// ID returns the ID of p in this table. It panics if p is not registered, as
// that is always a programming error.
func (t *Table) ID(p Packet) int32 {
	id, ok := t.ids[reflect.TypeOf(p)]

	if !ok {
		panic(fmt.Sprintf("packet: %T is not part of %s", p, t.name))
	}

	return id
}

// Decode decodes the payload of the packet with the given ID.
func (t *Table) Decode(id int32, payload []byte) (Packet, error) {
	typ, ok := t.types[id]

	if !ok {
		return nil, fmt.Errorf("%w 0x%02X in %s", ErrUnknownPacket, id, t.name)
	}

	p := reflect.New(typ).Interface().(Packet)
	r := protocol.NewReader(payload)
	p.Decode(r)

	if err := r.Finish(); err != nil {
		return nil, fmt.Errorf("packet: decoding %T: %w", p, err)
	}

	return p, nil
}

// Encode encodes p into a fresh payload.
func Encode(p Packet) []byte {
	w := protocol.NewWriter()
	p.Encode(w)

	return w.Bytes()
}

func writeNBT(w *protocol.Writer, tag nbt.Tag) {
	w.Raw(nbt.Append(nil, tag))
}

func readNBT(r *protocol.Reader) nbt.Tag {
	tag, n, err := nbt.Decode(r.Unread())

	if err != nil {
		r.Fail(err)
		return nil
	}

	r.Skip(n)

	return tag
}

func writeComponent(w *protocol.Writer, c text.Component) {
	writeNBT(w, c.NBT())
}

func readComponent(r *protocol.Reader) text.Component {
	tag := readNBT(r)

	if tag == nil {
		return text.Component{}
	}

	return text.FromNBT(tag)
}

// writeList writes a VarInt count followed by each item.
func writeList[T any](w *protocol.Writer, items []T, write func(T)) {
	w.VarInt(int32(len(items)))

	for _, item := range items {
		write(item)
	}
}

// readList reads a VarInt count of at most max items followed by each item.
// An empty list decodes as nil.
func readList[T any](r *protocol.Reader, max int, read func() T) []T {
	n := r.Length(max)

	if n == 0 {
		return nil
	}

	items := make([]T, 0, n)

	for i := 0; i < n && r.Err() == nil; i++ {
		items = append(items, read())
	}

	return items
}
