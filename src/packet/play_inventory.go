package packet

import "main/src/protocol"

// InventoryContainer is the container ID of the player's own inventory menu.
const InventoryContainer int32 = 0

// SetCarriedItem selects a hotbar slot, from 0 to 8.
type SetCarriedItem struct {
	Slot int16
}

func (p *SetCarriedItem) Encode(w *protocol.Writer) { w.Short(p.Slot) }
func (p *SetCarriedItem) Decode(r *protocol.Reader) { p.Slot = r.Short() }

// SetCreativeModeSlot sets an inventory slot from the creative inventory.
// Slots use the player inventory menu numbering: 9 to 35 are the main
// inventory, 36 to 44 the hotbar and 45 the off hand.
type SetCreativeModeSlot struct {
	Slot  int16
	Stack ItemStack
}

func (p *SetCreativeModeSlot) Encode(w *protocol.Writer) {
	w.Short(p.Slot)
	p.Stack.encodeDelimited(w)
}

func (p *SetCreativeModeSlot) Decode(r *protocol.Reader) {
	p.Slot = r.Short()
	p.Stack.decodeDelimited(r)
}

// PickItemFromBlock asks for the item of a block, sent when the player
// middle-clicks it. IncludeData is set when the pick key is used with Ctrl.
type PickItemFromBlock struct {
	Pos         BlockPos
	IncludeData bool
}

func (p *PickItemFromBlock) Encode(w *protocol.Writer) {
	p.Pos.encode(w)
	w.Bool(p.IncludeData)
}

func (p *PickItemFromBlock) Decode(r *protocol.Reader) {
	p.Pos.decode(r)
	p.IncludeData = r.Bool()
}

// SetHeldSlot selects a hotbar slot on the client, from 0 to 8.
type SetHeldSlot struct {
	Slot int32
}

func (p *SetHeldSlot) Encode(w *protocol.Writer) { w.VarInt(p.Slot) }
func (p *SetHeldSlot) Decode(r *protocol.Reader) { p.Slot = r.VarInt() }

// ContainerSetContent replaces every slot of a container and the stack held
// on the cursor. StateID numbers the container's revision for click packets.
type ContainerSetContent struct {
	ContainerID int32
	StateID     int32
	Items       []ItemStack
	Carried     ItemStack
}

func (p *ContainerSetContent) Encode(w *protocol.Writer) {
	w.VarInt(p.ContainerID)
	w.VarInt(p.StateID)
	writeList(w, p.Items, func(s ItemStack) { s.encode(w) })
	p.Carried.encode(w)
}

func (p *ContainerSetContent) Decode(r *protocol.Reader) {
	p.ContainerID = r.VarInt()
	p.StateID = r.VarInt()
	p.Items = readList(r, r.Remaining(), func() ItemStack {
		var s ItemStack
		s.decode(r)

		return s
	})
	p.Carried.decode(r)
}

// ContainerSetSlot changes one slot of a container.
type ContainerSetSlot struct {
	ContainerID int32
	StateID     int32
	Slot        int16
	Stack       ItemStack
}

func (p *ContainerSetSlot) Encode(w *protocol.Writer) {
	w.VarInt(p.ContainerID)
	w.VarInt(p.StateID)
	w.Short(p.Slot)
	p.Stack.encode(w)
}

func (p *ContainerSetSlot) Decode(r *protocol.Reader) {
	p.ContainerID = r.VarInt()
	p.StateID = r.VarInt()
	p.Slot = r.Short()
	p.Stack.decode(r)
}
