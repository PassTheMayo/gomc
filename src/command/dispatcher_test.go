package command

import (
	"bytes"
	"strings"
	"testing"

	"main/src/packet"
	"main/src/text"
)

type testSource struct {
	operator bool
	replies  []text.Component
}

func (s *testSource) Reply(msg text.Component) { s.replies = append(s.replies, msg) }
func (s *testSource) IsOperator() bool         { return s.operator }

func newDispatcher(ran *[]string) *Dispatcher {
	d := &Dispatcher{}
	d.Register(Literal("pregen",
		Integer("radius", 1, 100).Executes(func(ctx *Context) {
			*ran = append(*ran, "radius")

			if ctx.Int("radius") != 7 {
				panic("unexpected radius")
			}
		}),
		Literal("cancel").Executes(func(*Context) { *ran = append(*ran, "cancel") }),
	), true)
	d.Register(Literal("hello").Executes(func(*Context) { *ran = append(*ran, "hello") }), false)

	return d
}

func TestExecute(t *testing.T) {
	var ran []string
	d := newDispatcher(&ran)
	op := &testSource{operator: true}

	d.Execute(op, "pregen 7")
	d.Execute(op, "pregen cancel")
	d.Execute(op, "hello")

	if len(ran) != 3 || ran[0] != "radius" || ran[1] != "cancel" || ran[2] != "hello" || len(op.replies) != 0 {
		t.Fatalf("ran %v with replies %v", ran, op.replies)
	}

	errors := map[string]string{
		"":            "command.unknown.command",
		"nope":        "command.unknown.command",
		"pregen":      "command.unknown.command",
		"pregen abc":  "parsing.int.invalid",
		"pregen 0":    "argument.integer.low",
		"pregen 101":  "argument.integer.big",
		"pregen 7 8":  "command.unknown.argument",
		"hello there": "command.unknown.argument",
	}

	for line, key := range errors {
		src := &testSource{operator: true}
		d.Execute(src, line)

		if len(src.replies) != 1 || src.replies[0].Translate != key || src.replies[0].Color != text.Red {
			t.Errorf("%q replied %+v, want %s", line, src.replies, key)
		}
	}

	if len(ran) != 3 {
		t.Fatalf("invalid commands must not run, ran %v", ran)
	}
}

func TestRestricted(t *testing.T) {
	var ran []string
	d := newDispatcher(&ran)
	player := &testSource{}

	d.Execute(player, "pregen 7")

	if len(ran) != 0 || len(player.replies) != 1 || player.replies[0].Translate != "command.unknown.command" {
		t.Fatalf("a restricted command ran for a non-operator: ran %v, replies %+v", ran, player.replies)
	}

	tree := d.Tree(player)

	if len(tree.Nodes) != 2 || tree.Nodes[1].Name != "hello" {
		t.Fatalf("non-operators must only see unrestricted commands, got %+v", tree.Nodes)
	}
}

func TestTree(t *testing.T) {
	var ran []string
	tree := newDispatcher(&ran).Tree(&testSource{operator: true})

	root := tree.Nodes[tree.Root]
	pregen := tree.Nodes[root.Children[0]]
	radius := tree.Nodes[pregen.Children[0]]
	cancel := tree.Nodes[pregen.Children[1]]

	if root.Type != packet.NodeRoot || len(root.Children) != 2 {
		t.Fatalf("unexpected root %+v", root)
	}

	if pregen.Name != "pregen" || !pregen.Restricted || pregen.Executable {
		t.Fatalf("unexpected pregen node %+v", pregen)
	}

	if radius.Type != packet.NodeArgument || radius.Parser != packet.ParserInteger || !bytes.Equal(radius.Properties, packet.IntegerProperties(1, 100)) || !radius.Executable {
		t.Fatalf("unexpected radius node %+v", radius)
	}

	if cancel.Type != packet.NodeLiteral || cancel.Name != "cancel" || !cancel.Executable || cancel.Restricted {
		t.Fatalf("unexpected cancel node %+v", cancel)
	}

	// The tree must survive the network round trip.
	if _, err := packet.PlayClientbound.Decode(packet.PlayClientbound.ID(tree), packet.Encode(tree)); err != nil {
		t.Fatal(err)
	}
}

// restArgument takes every remaining word, like a message argument.
type restArgument struct{}

func (restArgument) Parse(r *Reader) (any, *text.Component) {
	var words []string

	for !r.AtEnd() {
		words = append(words, r.ReadWord())
	}

	return strings.Join(words, " "), nil
}

func (restArgument) Parser() (int32, []byte) { return 5, nil }

// New argument types plug in without changes to the dispatcher.
func TestCustomArgument(t *testing.T) {
	var got any
	d := &Dispatcher{}
	d.Register(Literal("say", Arg("message", restArgument{}).Executes(func(ctx *Context) { got = ctx.values["message"] })), false)

	d.Execute(&testSource{}, "say  hello   there world")

	if got != "hello there world" {
		t.Fatalf("message parsed as %q", got)
	}
}
