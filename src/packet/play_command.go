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

// Argument parser IDs from the minecraft:command_argument_type registry.
const (
	ParserInteger  int32 = 3
	ParserEntity   int32 = 6
	ParserGameMode int32 = 42
)

// Integer parser flags marking which bounds follow.
const (
	integerHasMin uint8 = 0x01
	integerHasMax uint8 = 0x02
)

// Entity parser flags, the only property of an entity argument.
const (
	EntitySingle      uint8 = 0x01
	EntityPlayersOnly uint8 = 0x02
)

var errUnsupportedNode = errors.New("packet: unsupported command node")

// parserProperties reads the properties of each supported argument parser.
// Decoding needs them to find where a node ends; encoding writes the bytes
// the argument produced.
var parserProperties = map[int32]func(r *protocol.Reader){
	ParserInteger: func(r *protocol.Reader) {
		bounds := r.UByte()

		if bounds&integerHasMin != 0 {
			r.Int()
		}

		if bounds&integerHasMax != 0 {
			r.Int()
		}
	},
	ParserEntity:   func(r *protocol.Reader) { r.UByte() },
	ParserGameMode: func(*protocol.Reader) {},
}

// IntegerProperties encodes the bounds of an integer argument. math.MinInt32
// and math.MaxInt32 mean unbounded.
func IntegerProperties(min, max int32) []byte {
	w := protocol.NewWriter()
	var bounds uint8

	if min != math.MinInt32 {
		bounds |= integerHasMin
	}

	if max != math.MaxInt32 {
		bounds |= integerHasMax
	}

	w.UByte(bounds)

	if bounds&integerHasMin != 0 {
		w.Int(min)
	}

	if bounds&integerHasMax != 0 {
		w.Int(max)
	}

	return w.Bytes()
}

// CommandNode is a node of the command tree the client uses to suggest and
// validate commands. Argument nodes name their parser and carry its encoded
// properties.
type CommandNode struct {
	Type       uint8
	Executable bool
	Restricted bool
	Children   []int32
	Name       string
	Parser     int32
	Properties []byte
}

// Commands sends the commands the player may use.
type Commands struct {
	Nodes []CommandNode
	Root  int32
}

func (p *Commands) Encode(w *protocol.Writer) {
	protocol.WriteList(w, p.Nodes, func(n CommandNode) {
		flags := n.Type

		if n.Executable {
			flags |= nodeExecutable
		}

		if n.Restricted {
			flags |= nodeRestricted
		}

		w.UByte(flags)
		protocol.WriteList(w, n.Children, w.VarInt)

		if n.Type == NodeRoot {
			return
		}

		w.String(n.Name)

		if n.Type == NodeArgument {
			w.VarInt(n.Parser)
			w.Raw(n.Properties)
		}
	})
	w.VarInt(p.Root)
}

func (p *Commands) Decode(r *protocol.Reader) {
	p.Nodes = protocol.ReadList(r, r.Remaining(), func() CommandNode {
		flags := r.UByte()
		n := CommandNode{
			Type:       flags & nodeTypeMask,
			Executable: flags&nodeExecutable != 0,
			Restricted: flags&nodeRestricted != 0,
			Children:   protocol.ReadList(r, r.Remaining(), r.VarInt),
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
			n.Parser = r.VarInt()
			readProperties, ok := parserProperties[n.Parser]

			if !ok {
				r.Fail(errUnsupportedNode)
				return n
			}

			start := r.Unread()
			readProperties(r)

			if size := len(start) - len(r.Unread()); size > 0 {
				n.Properties = start[:size]
			}
		}

		return n
	})
	p.Root = r.VarInt()
}
