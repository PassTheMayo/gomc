package command

import (
	"fmt"
	"strconv"
	"strings"

	"main/src/packet"
	"main/src/text"
)

// command is a registered top-level command.
type command struct {
	root       *Node
	restricted bool
}

// Dispatcher holds the commands of the server.
type Dispatcher struct {
	commands []command
}

// Register adds a command whose root must be a literal. Restricted commands
// are only visible to and usable by operators.
func (d *Dispatcher) Register(root *Node, restricted bool) {
	if !root.literal {
		panic("command: the root of a command must be a literal")
	}

	d.commands = append(d.commands, command{root: root, restricted: restricted})
}

func (d *Dispatcher) visible(src Source) []command {
	var out []command

	for _, c := range d.commands {
		if !c.restricted || src.IsOperator() {
			out = append(out, c)
		}
	}

	return out
}

// Execute parses and runs a command line typed without the leading slash.
// Errors are reported to the source with the client's own translations.
func (d *Dispatcher) Execute(src Source, line string) {
	words := strings.Fields(line)
	unknown := text.Translatable("command.unknown.command").Colored(text.Red)

	if len(words) == 0 {
		src.Reply(unknown)
		return
	}

	var node *Node

	for _, c := range d.visible(src) {
		if c.root.name == words[0] {
			node = c.root
		}
	}

	if node == nil {
		src.Reply(unknown)
		return
	}

	ctx := &Context{Source: src, ints: make(map[string]int32)}

	for _, word := range words[1:] {
		next, err := node.match(word, ctx)

		if err != nil {
			src.Reply(err.Colored(text.Red))
			return
		}

		node = next
	}

	if node.run == nil {
		src.Reply(unknown)
		return
	}

	node.run(ctx)
}

// match finds the child of n that accepts word, preferring literals, and
// stores argument values in ctx.
func (n *Node) match(word string, ctx *Context) (*Node, *text.Component) {
	for _, child := range n.children {
		if child.literal && child.name == word {
			return child, nil
		}
	}

	for _, child := range n.children {
		if child.literal {
			continue
		}

		v, err := strconv.ParseInt(word, 10, 32)

		var msg text.Component

		switch {
		case err != nil:
			msg = text.Translatable("parsing.int.invalid", text.Text(word))
		case v < int64(child.min):
			msg = text.Translatable("argument.integer.low", text.Text(fmt.Sprint(child.min)), text.Text(word))
		case v > int64(child.max):
			msg = text.Translatable("argument.integer.big", text.Text(fmt.Sprint(child.max)), text.Text(word))
		default:
			ctx.ints[child.name] = int32(v)
			return child, nil
		}

		return nil, &msg
	}

	msg := text.Translatable("command.unknown.argument")

	return nil, &msg
}

// Tree returns the command tree the source may use, for the client to
// suggest and validate commands.
func (d *Dispatcher) Tree(src Source) *packet.Commands {
	p := &packet.Commands{Nodes: []packet.CommandNode{{Type: packet.NodeRoot}}}

	var add func(n *Node, restricted bool) int32

	add = func(n *Node, restricted bool) int32 {
		index := int32(len(p.Nodes))
		node := packet.CommandNode{Name: n.name, Executable: n.run != nil, Restricted: restricted}

		if n.literal {
			node.Type = packet.NodeLiteral
		} else {
			node.Type, node.Parser, node.Min, node.Max = packet.NodeArgument, packet.ParserInteger, n.min, n.max
		}

		p.Nodes = append(p.Nodes, node)

		for _, child := range n.children {
			childIndex := add(child, false)
			p.Nodes[index].Children = append(p.Nodes[index].Children, childIndex)
		}

		return index
	}

	for _, c := range d.visible(src) {
		p.Nodes[0].Children = append(p.Nodes[0].Children, add(c.root, c.restricted))
	}

	return p
}
