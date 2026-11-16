package packet

import (
	"errors"

	"main/src/protocol"
)

var errItemComponents = errors.New("packet: components of server-sent item stacks cannot be decoded")

// ItemComponent is a data component of an item stack, kept as its encoded value.
type ItemComponent struct {
	Type int32
	Data []byte
}

// ItemStack is a stack of items. A zero count is an empty slot. The server
// never interprets components; it stores what clients send and sends it back.
type ItemStack struct {
	Count      int32
	Item       int32
	Components []ItemComponent
	Removed    []int32
}

// IsEmpty reports whether the stack is an empty slot.
func (s *ItemStack) IsEmpty() bool {
	return s.Count <= 0
}

// encode writes the stack in the form the server sends, with component values
// back to back.
func (s *ItemStack) encode(w *protocol.Writer) {
	s.write(w, false)
}

// encodeDelimited writes the stack in the form clients send, where every
// component value is length-prefixed so it can be read without knowing its type.
func (s *ItemStack) encodeDelimited(w *protocol.Writer) {
	s.write(w, true)
}

func (s *ItemStack) write(w *protocol.Writer, delimited bool) {
	w.VarInt(s.Count)

	if s.IsEmpty() {
		return
	}

	w.VarInt(s.Item)
	w.VarInt(int32(len(s.Components)))
	w.VarInt(int32(len(s.Removed)))

	for _, c := range s.Components {
		w.VarInt(c.Type)

		if delimited {
			w.ByteArray(c.Data)
		} else {
			w.Raw(c.Data)
		}
	}

	for _, t := range s.Removed {
		w.VarInt(t)
	}
}

// decode reads a server-sent stack. Its component values can only be told
// apart by knowing every component's encoding, so stacks with added
// components fail to decode.
func (s *ItemStack) decode(r *protocol.Reader) {
	s.read(r, false)
}

// decodeDelimited reads a client-sent stack.
func (s *ItemStack) decodeDelimited(r *protocol.Reader) {
	s.read(r, true)
}

func (s *ItemStack) read(r *protocol.Reader, delimited bool) {
	if s.Count = r.VarInt(); s.IsEmpty() {
		return
	}

	s.Item = r.VarInt()
	added, removed := r.Length(r.Remaining()), r.Length(r.Remaining())

	if added > 0 && !delimited {
		r.Fail(errItemComponents)
		return
	}

	for i := 0; i < added && r.Err() == nil; i++ {
		s.Components = append(s.Components, ItemComponent{Type: r.VarInt(), Data: r.ByteArray(r.Remaining())})
	}

	for i := 0; i < removed && r.Err() == nil; i++ {
		s.Removed = append(s.Removed, r.VarInt())
	}
}
