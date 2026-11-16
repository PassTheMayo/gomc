package inventory

import (
	"math"
	"slices"

	"main/src/item"
	"main/src/packet"
)

const (
	// maxSlotStack is the most items any slot holds.
	maxSlotStack = 99

	// unlimited is a removal limit that never applies.
	unlimited = math.MaxInt32
)

// slot is one slot of a container window.
type slot interface {
	get() item.Stack
	set(s item.Stack)
	mayPlace(s item.Stack) bool
	mayPickup(creative bool) bool
	maxStackSize() int32
}

// menu is a container window: its slots, the stack held on the cursor and a
// drag in progress. Clicks follow vanilla's AbstractContainerMenu.
type menu struct {
	slots    []slot
	carried  item.Stack
	revision int32
	drag     drag

	// quickMove moves a slot's stack elsewhere on shift-click and returns
	// what it moved, or an empty stack if nothing moved.
	quickMove func(i int) item.Stack

	// output reports slots items can't be dragged into or gathered from,
	// like a crafting result.
	output func(i int) bool

	// hotbar returns the slot the swap key with a hotbar number, or 40 for
	// the off hand, exchanges with.
	hotbar func(button int) slot

	// give puts a stack into the player's inventory and returns what didn't
	// fit.
	give func(s item.Stack) item.Stack
}

// drag is the state of a drag across slots, which clients send in steps:
// start, one step per slot, then end.
type drag struct {
	status int
	kind   int
	slots  []int
}

// Drag kinds.
const (
	dragSplit = iota
	dragOne
	dragClone
)

// click applies a click to the menu and returns the stacks the player threw
// out of it, which drop into the world.
func (m *menu) click(index, button int, mode int32, creative bool) []item.Stack {
	var thrown []item.Stack

	switch {
	case index != int(packet.ClickOutside) && index >= len(m.slots):
	case mode == packet.ClickQuickCraft:
		m.dragStep(index, button, creative)
	case m.drag.status != 0:
		m.drag = drag{}
	case (mode == packet.ClickPickup || mode == packet.ClickQuickMove) && (button == 0 || button == 1):
		thrown = m.pickup(index, button, mode, creative)
	case mode == packet.ClickSwap && (button >= 0 && button < hotbarSize || button == offHandSlot) && index >= 0:
		thrown = m.swap(index, button, creative)
	case mode == packet.ClickClone && creative && m.carried.IsEmpty() && index >= 0:
		if s := m.slots[index].get(); !s.IsEmpty() {
			m.carried = s.WithCount(s.MaxStackSize())
		}
	case mode == packet.ClickThrow && m.carried.IsEmpty() && index >= 0:
		thrown = m.throw(index, button, creative)
	case mode == packet.ClickPickupAll && index >= 0:
		m.gather(index, button, creative)
	}

	return thrown
}

// pickup handles left and right clicks: picking up, placing, swapping and
// shift-clicking stacks, and dropping the cursor outside the window.
func (m *menu) pickup(index, button int, mode int32, creative bool) []item.Stack {
	primary := button == 0

	if index == int(packet.ClickOutside) {
		switch {
		case m.carried.IsEmpty():
			return nil
		case primary:
			thrown := m.carried
			m.carried = item.Stack{}

			return []item.Stack{thrown}
		default:
			m.carried.Count--

			return []item.Stack{m.carried.WithCount(1)}
		}
	}

	if index < 0 {
		return nil
	}

	s := m.slots[index]

	if mode == packet.ClickQuickMove {
		if !s.mayPickup(creative) {
			return nil
		}

		for moved := m.quickMove(index); !moved.IsEmpty() && s.get().Item == moved.Item; {
			moved = m.quickMove(index)
		}

		return nil
	}

	current, carried := s.get(), m.carried

	switch {
	case current.IsEmpty():
		if !carried.IsEmpty() {
			m.carried = insert(s, carried, pick(primary, carried.Count, 1))
		}
	case !s.mayPickup(creative):
	case carried.IsEmpty():
		if taken, ok := tryRemove(s, pick(primary, current.Count, (current.Count+1)/2), unlimited, creative); ok {
			m.carried = taken
		}
	case s.mayPlace(carried):
		if item.SameItemSameComponents(current, carried) {
			m.carried = insert(s, carried, pick(primary, carried.Count, 1))
		} else if carried.Count <= maxFor(s, carried) {
			m.carried = current
			s.set(carried)
		}
	case item.SameItemSameComponents(current, carried):
		if taken, ok := tryRemove(s, current.Count, carried.MaxStackSize()-carried.Count, creative); ok {
			m.carried.Count += taken.Count
		}
	}

	return nil
}

// swap exchanges a slot with a hotbar slot or the off hand, for the number
// keys and the swap key.
func (m *menu) swap(index, button int, creative bool) []item.Stack {
	s, hand := m.slots[index], m.hotbar(button)
	held, current := hand.get(), s.get()

	switch {
	case held.IsEmpty() && current.IsEmpty():
	case held.IsEmpty():
		if s.mayPickup(creative) {
			hand.set(current)
			s.set(item.Stack{})
		}
	case current.IsEmpty():
		if !s.mayPlace(held) {
			break
		}

		if limit := maxFor(s, held); held.Count > limit {
			s.set(held.WithCount(limit))
			hand.set(held.WithCount(held.Count - limit))
		} else {
			hand.set(item.Stack{})
			s.set(held)
		}
	case s.mayPickup(creative) && s.mayPlace(held):
		limit := maxFor(s, held)

		if held.Count <= limit {
			hand.set(current)
			s.set(held)

			break
		}

		s.set(held.WithCount(limit))
		hand.set(held.WithCount(held.Count - limit))

		if rest := m.give(current); !rest.IsEmpty() {
			return []item.Stack{rest}
		}
	}

	return nil
}

// throw drops one item, or with Ctrl the whole stack, from a slot.
func (m *menu) throw(index, button int, creative bool) []item.Stack {
	s := m.slots[index]
	count := int32(1)

	if button == 1 {
		count = s.get().Count
	}

	taken, ok := tryRemove(s, count, unlimited, creative)

	if !ok {
		return nil
	}

	thrown := []item.Stack{taken}

	for button == 1 && s.get().Item == taken.Item {
		if taken, ok = tryRemove(s, count, unlimited, creative); !ok {
			break
		}

		thrown = append(thrown, taken)
	}

	return thrown
}

// gather collects stacks matching the cursor onto it, on double click:
// partial stacks first, then full ones.
func (m *menu) gather(index, button int, creative bool) {
	s := m.slots[index]

	if m.carried.IsEmpty() || !s.get().IsEmpty() && s.mayPickup(creative) {
		return
	}

	start, step := 0, 1

	if button != 0 {
		start, step = len(m.slots)-1, -1
	}

	for pass := range 2 {
		for i := start; i >= 0 && i < len(m.slots) && m.carried.Count < m.carried.MaxStackSize(); i += step {
			other := m.slots[i]
			stack := other.get()

			if stack.IsEmpty() || !canQuickReplace(other, m.carried, true) || !other.mayPickup(creative) || m.output(i) {
				continue
			}

			if pass == 0 && stack.Count == stack.MaxStackSize() {
				continue
			}

			if taken, ok := tryRemove(other, stack.Count, m.carried.MaxStackSize()-m.carried.Count, creative); ok {
				m.carried.Count += taken.Count
			}
		}
	}
}

// dragStep advances a drag: the header in the low two bits of button says
// whether it starts, adds a slot or ends, and the next two bits its kind.
func (m *menu) dragStep(index, button int, creative bool) {
	previous := m.drag.status
	m.drag.status = button & 3

	switch {
	case (previous != 1 || m.drag.status != 2) && previous != m.drag.status, m.carried.IsEmpty():
		m.drag = drag{}
	case m.drag.status == 0:
		m.drag.kind = (button >> 2) & 3

		if m.drag.kind == dragSplit || m.drag.kind == dragOne || m.drag.kind == dragClone && creative {
			m.drag.status = 1
			m.drag.slots = nil
		} else {
			m.drag = drag{}
		}
	case m.drag.status == 1:
		if index >= 0 && m.canDragTo(index, m.carried) && (m.drag.kind == dragClone || int(m.carried.Count) > len(m.drag.slots)) && !slices.Contains(m.drag.slots, index) {
			m.drag.slots = append(m.drag.slots, index)
		}
	case m.drag.status == 2:
		m.finishDrag(creative)
	default:
		m.drag = drag{}
	}
}

// finishDrag spreads the cursor stack over the dragged slots. A drag over
// one slot is a click with the drag kind as the button.
func (m *menu) finishDrag(creative bool) {
	d := m.drag
	m.drag = drag{}

	switch len(d.slots) {
	case 0:
		return
	case 1:
		m.click(d.slots[0], d.kind, packet.ClickPickup, creative)
		return
	}

	source := m.carried
	remaining := source.Count

	for _, i := range d.slots {
		s := m.slots[i]

		if !m.canDragTo(i, m.carried) || d.kind != dragClone && m.carried.Count < int32(len(d.slots)) {
			continue
		}

		var have int32

		if current := s.get(); !current.IsEmpty() {
			have = current.Count
		}

		placed := min(dragShare(d, source)+have, maxFor(s, source))
		remaining -= placed - have
		s.set(source.WithCount(placed))
	}

	m.carried = source.WithCount(remaining)
}

// canDragTo reports whether a drag may place a stack in a slot.
func (m *menu) canDragTo(index int, s item.Stack) bool {
	return canQuickReplace(m.slots[index], s, true) && m.slots[index].mayPlace(s) && !m.output(index)
}

// dragShare returns how many items a drag puts in each slot.
func dragShare(d drag, s item.Stack) int32 {
	switch d.kind {
	case dragSplit:
		return s.Count / int32(len(d.slots))
	case dragOne:
		return 1
	}

	return s.MaxStackSize()
}

// moveTo moves as much of a stack as fits into slots from start up to end,
// or backwards, filling matching stacks before empty slots. It returns what
// is left and whether anything moved.
func (m *menu) moveTo(s item.Stack, start, end int, backwards bool) (item.Stack, bool) {
	moved := false
	order := func(yield func(int) bool) {
		for k := range end - start {
			i := start + k

			if backwards {
				i = end - 1 - k
			}

			if !yield(i) {
				return
			}
		}
	}

	if s.IsStackable() {
		for i := range order {
			if s.IsEmpty() {
				break
			}

			target := m.slots[i]
			current := target.get()

			if current.IsEmpty() || !item.SameItemSameComponents(s, current) {
				continue
			}

			limit := maxFor(target, current)

			if total := current.Count + s.Count; total <= limit {
				target.set(current.WithCount(total))
				s.Count = 0
				moved = true
			} else if current.Count < limit {
				s.Count -= limit - current.Count
				target.set(current.WithCount(limit))
				moved = true
			}
		}
	}

	if s.IsEmpty() {
		return s, moved
	}

	for i := range order {
		target := m.slots[i]

		if target.get().IsEmpty() && target.mayPlace(s) {
			n := min(s.Count, maxFor(target, s))
			target.set(s.WithCount(n))
			s.Count -= n

			return s, true
		}
	}

	return s, moved
}

// nextRevision numbers a change the server sends the client.
func (m *menu) nextRevision() int32 {
	m.revision = (m.revision + 1) & 32767

	return m.revision
}

// maxFor returns how many of a stack's item fit in a slot.
func maxFor(s slot, stack item.Stack) int32 {
	return min(s.maxStackSize(), stack.MaxStackSize())
}

// insert moves up to n items of a stack into a slot and returns the rest.
func insert(s slot, stack item.Stack, n int32) item.Stack {
	if stack.IsEmpty() || !s.mayPlace(stack) {
		return stack
	}

	current := s.get()
	n = min(n, stack.Count, maxFor(s, stack)-current.Count)

	switch {
	case n <= 0:
	case current.IsEmpty():
		s.set(stack.WithCount(n))
		stack.Count -= n
	case item.SameItemSameComponents(current, stack):
		s.set(current.WithCount(current.Count + n))
		stack.Count -= n
	}

	return stack
}

// tryRemove takes up to min(count, limit) items from a slot. Slots whose
// stack can't be put back may only be emptied.
func tryRemove(s slot, count, limit int32, creative bool) (item.Stack, bool) {
	current := s.get()

	if !s.mayPickup(creative) || current.IsEmpty() {
		return item.Stack{}, false
	}

	if !s.mayPlace(current) && limit < current.Count {
		return item.Stack{}, false
	}

	n := min(count, limit, current.Count)

	if n <= 0 {
		return item.Stack{}, false
	}

	s.set(current.WithCount(current.Count - n))

	return current.WithCount(n), true
}

// canQuickReplace reports whether a stack may go into a slot by dragging or
// gathering: the slot is empty, or holds the same item with room for it.
func canQuickReplace(s slot, stack item.Stack, sizeMatters bool) bool {
	current := s.get()

	if current.IsEmpty() {
		return true
	}

	if !item.SameItemSameComponents(stack, current) {
		return false
	}

	extra := stack.Count

	if sizeMatters {
		extra = 0
	}

	return current.Count+extra <= stack.MaxStackSize()
}

func pick(primary bool, ifPrimary, otherwise int32) int32 {
	if primary {
		return ifPrimary
	}

	return otherwise
}
