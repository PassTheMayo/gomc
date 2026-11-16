// Package item provides item stacks and their data components.
package item

import (
	"bytes"
	"slices"

	"main/src/registry"
)

// enchantments is the data component type holding a stack's enchantments.
var enchantments = registry.MustGet(registry.DataComponentType).MustID("minecraft:enchantments")

// Component is a data component of a stack, kept as its encoded value. The
// server never interprets most components; it stores what clients send and
// sends it back.
type Component struct {
	Type int32
	Data []byte
}

// Stack is a stack of items. A zero count is an empty slot. Components and
// Removed are the stack's changes to its item's default components.
type Stack struct {
	Count      int32
	Item       int32
	Components []Component
	Removed    []int32
}

// Of returns a stack of count items with default components.
func Of(item, count int32) Stack {
	return Stack{Count: count, Item: item}
}

// IsEmpty reports whether the stack is an empty slot.
func (s Stack) IsEmpty() bool {
	return s.Count <= 0
}

// IsEnchanted reports whether the stack has an enchantments component with
// any entries. Its value starts with the VarInt entry count, which encodes as
// a zero byte only when it is zero.
func (s Stack) IsEnchanted() bool {
	data, ok := s.component(enchantments)

	return ok && len(data) > 0 && data[0] != 0
}

// Equal reports whether two stacks hold the same number of the same item with
// the same components.
func Equal(a, b Stack) bool {
	return a.Count == b.Count && (a.IsEmpty() || SameItemSameComponents(a, b))
}

// SameItemSameComponents reports whether two non-empty stacks hold the same
// item with the same component changes, ignoring their counts and the order
// components are listed in.
func SameItemSameComponents(a, b Stack) bool {
	if a.IsEmpty() || b.IsEmpty() || a.Item != b.Item || len(a.Components) != len(b.Components) || len(a.Removed) != len(b.Removed) {
		return false
	}

	for _, c := range a.Components {
		if !slices.ContainsFunc(b.Components, func(o Component) bool { return o.Type == c.Type && bytes.Equal(o.Data, c.Data) }) {
			return false
		}
	}

	for _, t := range a.Removed {
		if !slices.Contains(b.Removed, t) {
			return false
		}
	}

	return true
}
