package server

import (
	"reflect"

	"main/src/packet"
	"main/src/text"
)

// router connects play packets and join steps to the features that handle
// them. Each feature registers itself from its own file while the server is
// created, so adding a feature never means editing a central switch.
// Registration happens before any connection is accepted, so the router is
// read without locking.
type router struct {
	handlers   map[reflect.Type][]handlerFunc
	joinSteps  []func(p *Player)
	leaveSteps []func(p *Player)
}

type handlerFunc func(p *Player, pk packet.Packet) error

// kickError is returned by a handler to disconnect the player that sent the
// packet, with a reason shown to it.
type kickError struct {
	reason text.Component
}

func (e *kickError) Error() string {
	return e.reason.PlainText()
}

// on registers fn to handle play packets of type T. A packet type may have
// several handlers, which run in registration order.
func on[T packet.Packet](r *router, fn func(p *Player, pk T)) {
	onChecked(r, func(p *Player, pk T) error {
		fn(p, pk)

		return nil
	})
}

// onChecked is like on, but an error returned by fn disconnects the player
// and stops the remaining handlers.
func onChecked[T packet.Packet](r *router, fn func(p *Player, pk T) error) {
	if r.handlers == nil {
		r.handlers = make(map[reflect.Type][]handlerFunc)
	}

	t := reflect.TypeFor[T]()
	r.handlers[t] = append(r.handlers[t], func(p *Player, pk packet.Packet) error { return fn(p, pk.(T)) })
}

// onJoin adds a step to the join sequence. Steps run in registration order,
// after the login packet and before the player is added to the tab list.
func (r *router) onJoin(step func(p *Player)) {
	r.joinSteps = append(r.joinSteps, step)
}

// onLeave adds a step run when a player leaves, before its data is saved.
func (r *router) onLeave(step func(p *Player)) {
	r.leaveSteps = append(r.leaveSteps, step)
}

// dispatch runs the handlers of a packet. Packets without handlers are ignored.
func (r *router) dispatch(p *Player, pk packet.Packet) error {
	for _, handle := range r.handlers[reflect.TypeOf(pk)] {
		if err := handle(p, pk); err != nil {
			return err
		}
	}

	return nil
}

// join runs the join steps for p.
func (r *router) join(p *Player) {
	for _, step := range r.joinSteps {
		step(p)
	}
}

// leave runs the leave steps for p.
func (r *router) leave(p *Player) {
	for _, step := range r.leaveSteps {
		step(p)
	}
}
