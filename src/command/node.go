// Package command implements chat commands as trees of literals and
// arguments, following the Brigadier model the client uses to suggest and
// validate commands as they are typed.
package command

// Node is a literal word or an argument in a command tree.
type Node struct {
	name     string
	arg      Argument
	run      func(ctx *Context)
	children []*Node
}

// Literal returns a node matching the exact word name.
func Literal(name string, children ...*Node) *Node {
	return &Node{name: name, children: children}
}

// Arg returns a node parsed by arg, whose value is stored in the context under
// name.
func Arg(name string, arg Argument, children ...*Node) *Node {
	return &Node{name: name, arg: arg, children: children}
}

// Executes makes the command valid when it ends at this node.
func (n *Node) Executes(run func(ctx *Context)) *Node {
	n.run = run

	return n
}

func (n *Node) isLiteral() bool {
	return n.arg == nil
}
