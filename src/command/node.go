// Package command implements chat commands as trees of literals and
// arguments, following the Brigadier model the client uses to suggest and
// validate commands as they are typed.
package command

// Node is a literal word or an argument in a command tree.
type Node struct {
	name     string
	literal  bool
	min, max int32
	run      func(ctx *Context)
	children []*Node
}

// Literal returns a node matching the exact word name.
func Literal(name string, children ...*Node) *Node {
	return &Node{name: name, literal: true, children: children}
}

// Integer returns an argument node accepting whole numbers in [min, max],
// readable from the context under name.
func Integer(name string, min, max int32, children ...*Node) *Node {
	return &Node{name: name, min: min, max: max, children: children}
}

// Executes makes the command valid when it ends at this node.
func (n *Node) Executes(run func(ctx *Context)) *Node {
	n.run = run

	return n
}
