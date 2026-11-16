// Package inventory provides item containers and the player inventory built
// on them.
package inventory

import "main/src/item"

// Container is a fixed number of item slots, the storage behind every
// inventory: players' and, later, chests' and other blocks' and entities'.
// It isn't safe for concurrent use.
type Container struct {
	slots []item.Stack
}

// NewContainer returns an empty container with size slots.
func NewContainer(size int) *Container {
	return &Container{slots: make([]item.Stack, size)}
}

// Get returns the stack in slot i.
func (c *Container) Get(i int) item.Stack {
	return c.slots[i]
}

// Set puts a stack in slot i. Empty stacks are stored as the zero stack.
func (c *Container) Set(i int, s item.Stack) {
	if s.IsEmpty() {
		s = item.Stack{}
	}

	c.slots[i] = s
}

// Swap exchanges the stacks in two slots.
func (c *Container) Swap(i, j int) {
	c.slots[i], c.slots[j] = c.slots[j], c.slots[i]
}

// Find returns the first slot from from up to, but not including, to whose
// stack matches, or -1.
func (c *Container) Find(from, to int, match func(item.Stack) bool) int {
	for i := from; i < to; i++ {
		if match(c.slots[i]) {
			return i
		}
	}

	return -1
}

// isEmptySlot matches empty slots, for Find.
func isEmptySlot(s item.Stack) bool {
	return s.IsEmpty()
}
