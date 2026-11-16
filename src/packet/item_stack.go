package packet

import (
	"errors"
	"slices"

	"main/src/item"
	"main/src/protocol"
)

var errItemComponents = errors.New("packet: components of server-sent item stacks cannot be decoded")

// writeStack writes a stack in the form the server sends, with component
// values back to back.
func writeStack(w *protocol.Writer, s item.Stack) {
	writeStackForm(w, s, false)
}

// writeDelimitedStack writes a stack in the form clients send, where every
// component value is length-prefixed so it can be read without knowing its
// type.
func writeDelimitedStack(w *protocol.Writer, s item.Stack) {
	writeStackForm(w, s, true)
}

func writeStackForm(w *protocol.Writer, s item.Stack, delimited bool) {
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

// readStack reads a server-sent stack. Its component values can only be told
// apart by knowing every component's encoding, so stacks with added
// components fail to decode.
func readStack(r *protocol.Reader) item.Stack {
	return readStackForm(r, false)
}

// readDelimitedStack reads a client-sent stack.
func readDelimitedStack(r *protocol.Reader) item.Stack {
	return readStackForm(r, true)
}

func readStackForm(r *protocol.Reader, delimited bool) item.Stack {
	var s item.Stack

	if s.Count = r.VarInt(); s.IsEmpty() {
		return s
	}

	s.Item = r.VarInt()
	added, removed := r.Length(r.Remaining()), r.Length(r.Remaining())

	if added > 0 && !delimited {
		r.Fail(errItemComponents)
		return s
	}

	for i := 0; i < added && r.Err() == nil; i++ {
		s.Components = append(s.Components, item.Component{Type: r.VarInt(), Data: r.ByteArray(r.Remaining())})
	}

	for i := 0; i < removed && r.Err() == nil; i++ {
		s.Removed = append(s.Removed, r.VarInt())
	}

	return s
}

// HashedStack is how clients describe a stack in click packets: its item and
// count, with the values of changed components replaced by hashes. A zero
// count is an empty slot.
type HashedStack struct {
	Item       int32
	Count      int32
	Components []HashedComponent
	Removed    []int32
}

// HashedComponent is a changed component and the hash of its value.
type HashedComponent struct {
	Type int32
	Hash int32
}

// maxHashedComponents bounds the component lists of a hashed stack.
const maxHashedComponents = 256

// Matches reports whether the hashed stack has the item and count of s and
// changes the same components. Component values can't be compared without
// vanilla's hashing.
func (h HashedStack) Matches(s item.Stack) bool {
	if h.Count <= 0 || s.IsEmpty() {
		return h.Count <= 0 && s.IsEmpty()
	}

	if h.Item != s.Item || h.Count != s.Count || len(h.Components) != len(s.Components) || len(h.Removed) != len(s.Removed) {
		return false
	}

	for _, c := range h.Components {
		if !slices.ContainsFunc(s.Components, func(o item.Component) bool { return o.Type == c.Type }) {
			return false
		}
	}

	for _, t := range h.Removed {
		if !slices.Contains(s.Removed, t) {
			return false
		}
	}

	return true
}

func (h *HashedStack) encode(w *protocol.Writer) {
	w.Bool(h.Count > 0)

	if h.Count <= 0 {
		return
	}

	w.VarInt(h.Item)
	w.VarInt(h.Count)
	protocol.WriteList(w, h.Components, func(c HashedComponent) {
		w.VarInt(c.Type)
		w.Int(c.Hash)
	})
	protocol.WriteList(w, h.Removed, w.VarInt)
}

func (h *HashedStack) decode(r *protocol.Reader) {
	if !r.Bool() {
		return
	}

	h.Item = r.VarInt()
	h.Count = r.VarInt()
	h.Components = protocol.ReadList(r, maxHashedComponents, func() HashedComponent {
		return HashedComponent{Type: r.VarInt(), Hash: r.Int()}
	})
	h.Removed = protocol.ReadList(r, maxHashedComponents, r.VarInt)
}
