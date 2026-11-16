package packet

import (
	"errors"

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
