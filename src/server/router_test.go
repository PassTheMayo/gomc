package server

import (
	"errors"
	"slices"
	"testing"

	"main/src/packet"
	"main/src/text"
)

func TestRouter(t *testing.T) {
	var r router
	var calls []string

	on(&r, func(_ *Player, pk *packet.KeepAlive) { calls = append(calls, "first") })
	on(&r, func(_ *Player, pk *packet.KeepAlive) { calls = append(calls, "second") })
	r.onJoin(func(*Player) { calls = append(calls, "join a") })
	r.onJoin(func(*Player) { calls = append(calls, "join b") })

	kick := &kickError{reason: text.Text("bad")}
	onChecked(&r, func(*Player, *packet.SetCarriedItem) error { return kick })
	on(&r, func(*Player, *packet.SetCarriedItem) { calls = append(calls, "after kick") })

	// Every handler of a packet type runs in registration order.
	if err := r.dispatch(nil, &packet.KeepAlive{}); err != nil {
		t.Fatal(err)
	}

	// Packets without handlers are ignored.
	if err := r.dispatch(nil, &packet.PlayerLoaded{}); err != nil {
		t.Fatal(err)
	}

	// An error stops the remaining handlers.
	if err := r.dispatch(nil, &packet.SetCarriedItem{}); !errors.Is(err, kick) {
		t.Fatalf("dispatch returned %v, want the kick error", err)
	}

	r.join(nil)

	if want := []string{"first", "second", "join a", "join b"}; !slices.Equal(calls, want) {
		t.Fatalf("calls were %v, want %v", calls, want)
	}
}
