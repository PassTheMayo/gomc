package command

import (
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
	if !root.isLiteral() {
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
	r := NewReader(line)
	name := r.ReadWord()
	unknown := text.Translatable("command.unknown.command").Colored(text.Red)

	var node *Node

	for _, c := range d.visible(src) {
		if c.root.name == name {
			node = c.root
		}
	}

	if node == nil {
		src.Reply(unknown)
		return
	}

	ctx := &Context{Source: src, values: make(map[string]any)}

	for !r.AtEnd() {
		next, err := node.parseChild(r, ctx)

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

// parseChild reads the next part of the command with the child of n that
// accepts it, preferring literals, and stores argument values in ctx.
func (n *Node) parseChild(r *Reader, ctx *Context) (*Node, *text.Component) {
	word := r.PeekWord()

	for _, child := range n.children {
		if child.isLiteral() && child.name == word {
			r.ReadWord()
			return child, nil
		}
	}

	for _, child := range n.children {
		if child.isLiteral() {
			continue
		}

		v, err := child.arg.Parse(r)

		if err != nil {
			return nil, err
		}

		ctx.values[child.name] = v

		return child, nil
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

		if n.isLiteral() {
			node.Type = packet.NodeLiteral
		} else {
			node.Type = packet.NodeArgument
			node.Parser, node.Properties = n.arg.Parser()
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
