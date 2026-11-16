package command

import (
	"fmt"
	"strconv"

	"main/src/packet"
	"main/src/text"
)

// Argument parses one argument of a command and describes it to the client.
// New argument types implement it without changes to the dispatcher.
type Argument interface {
	// Parse reads the argument's value from r. On failure it returns the
	// message shown to whoever ran the command.
	Parse(r *Reader) (any, *text.Component)

	// Parser returns the brigadier parser ID and its encoded properties for
	// the command tree sent to clients.
	Parser() (id int32, properties []byte)
}

// integerArgument is a whole number in [min, max].
type integerArgument struct {
	min, max int32
}

// Integer returns an argument node accepting whole numbers in [min, max],
// readable from the context with Context.Int.
func Integer(name string, min, max int32, children ...*Node) *Node {
	return Arg(name, integerArgument{min: min, max: max}, children...)
}

func (a integerArgument) Parse(r *Reader) (any, *text.Component) {
	word := r.ReadWord()
	v, err := strconv.ParseInt(word, 10, 32)

	var msg text.Component

	switch {
	case err != nil:
		msg = text.Translatable("parsing.int.invalid", text.Text(word))
	case v < int64(a.min):
		msg = text.Translatable("argument.integer.low", text.Text(fmt.Sprint(a.min)), text.Text(word))
	case v > int64(a.max):
		msg = text.Translatable("argument.integer.big", text.Text(fmt.Sprint(a.max)), text.Text(word))
	default:
		return int32(v), nil
	}

	return nil, &msg
}

func (a integerArgument) Parser() (int32, []byte) {
	return packet.ParserInteger, packet.IntegerProperties(a.min, a.max)
}
