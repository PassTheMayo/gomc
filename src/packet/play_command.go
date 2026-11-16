package packet

import (
	"errors"
	"math"

	"main/src/protocol"
)

// ChatCommand is a command typed by the player, without the leading slash.
type ChatCommand struct {
	Command string
}

func (p *ChatCommand) Encode(w *protocol.Writer) { w.String(p.Command) }
func (p *ChatCommand) Decode(r *protocol.Reader) { p.Command = r.String(32767) }

// Command node types.
const (
	NodeRoot     uint8 = 0
	NodeLiteral  uint8 = 1
	NodeArgument uint8 = 2
)

// Command node flags, combined with the node type in the first byte.
const (
	nodeTypeMask    uint8 = 0x03
	nodeExecutable  uint8 = 0x04
	nodeRedirect    uint8 = 0x08
	nodeSuggestions uint8 = 0x10
	nodeRestricted  uint8 = 0x20
)

// ParserInteger is the ID of the brigadier:integer argument parser.
const ParserInteger int32 = 3

// Integer parser flags marking which bounds follow.
const (
	integerHasMin uint8 = 0x01
	integerHasMax uint8 = 0x02
)

var errUnsupportedNode = errors.New("packet: unsupported command node")

// CommandNode is a node of the command tree the client uses to suggest and
// validate commands. Only literal nodes and integer arguments are supported,
// which is all the server's commands need. Min and Max hold the integer
// bounds; math.MinInt32 and math.MaxInt32 mean unbounded.
type CommandNode struct {
	Type       uint8
	Executable bool
	Restricted bool
	Children   []int32
	Name       string
	Parser     int32
	Min, Max   int32
}

// Commands sends the commands the player may use.
type Commands struct {
	Nodes []CommandNode
	Root  int32
}

func (p *Commands) Encode(w *protocol.Writer) {
	writeList(w, p.Nodes, func(n CommandNode) {
		flags := n.Type

		if n.Executable {
			flags |= nodeExecutable
		}

		if n.Restricted {
			flags |= nodeRestricted
		}

		w.UByte(flags)
		writeList(w, n.Children, w.VarInt)

		if n.Type == NodeRoot {
			return
		}

		w.String(n.Name)

		if n.Type == NodeArgument {
			w.VarInt(n.Parser)

			var bounds uint8

			if n.Min != math.MinInt32 {
				bounds |= integerHasMin
			}

			if n.Max != math.MaxInt32 {
				bounds |= integerHasMax
			}

			w.UByte(bounds)

			if bounds&integerHasMin != 0 {
				w.Int(n.Min)
			}

			if bounds&integerHasMax != 0 {
				w.Int(n.Max)
			}
		}
	})
	w.VarInt(p.Root)
}

func (p *Commands) Decode(r *protocol.Reader) {
	p.Nodes = readList(r, r.Remaining(), func() CommandNode {
		flags := r.UByte()
		n := CommandNode{
			Type:       flags & nodeTypeMask,
			Executable: flags&nodeExecutable != 0,
			Restricted: flags&nodeRestricted != 0,
			Children:   readList(r, r.Remaining(), r.VarInt),
		}

		if flags&(nodeRedirect|nodeSuggestions) != 0 {
			r.Fail(errUnsupportedNode)
			return n
		}

		if n.Type == NodeRoot {
			return n
		}

		n.Name = r.String(32767)

		if n.Type == NodeArgument {
			if n.Parser = r.VarInt(); n.Parser != ParserInteger {
				r.Fail(errUnsupportedNode)
				return n
			}

			bounds := r.UByte()
			n.Min, n.Max = math.MinInt32, math.MaxInt32

			if bounds&integerHasMin != 0 {
				n.Min = r.Int()
			}

			if bounds&integerHasMax != 0 {
				n.Max = r.Int()
			}
		}

		return n
	})
	p.Root = r.VarInt()
}
