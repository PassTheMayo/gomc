package server

import (
	"cmp"
	"fmt"
	"slices"

	"main/src/command"
	"main/src/text"
)

// registerListCommand adds /list, which anyone may use to see who is online,
// and /list uuids, which adds each player's UUID.
func (s *Server) registerListCommand() {
	name := func(p *Player) text.Component { return text.Text(p.Profile.Name) }

	nameAndID := func(p *Player) text.Component {
		return text.Translatable("commands.list.nameAndId", text.Text(p.Profile.Name), text.Text(p.Profile.ID.String()))
	}

	s.commands.Register(command.Literal("list",
		command.Literal("uuids").Executes(func(ctx *command.Context) { s.listPlayers(ctx, nameAndID) }),
	).Executes(func(ctx *command.Context) { s.listPlayers(ctx, name) }), false)
}

// listPlayers replies with the number of players, the player limit and the
// players in the order they joined, each shown by format.
func (s *Server) listPlayers(ctx *command.Context, format func(p *Player) text.Component) {
	players := s.joinedPlayers()

	// Entity IDs are handed out as players join.
	slices.SortFunc(players, func(a, b *Player) int { return cmp.Compare(a.EntityID, b.EntityID) })

	parts := make([]text.Component, len(players))

	for i, p := range players {
		parts[i] = format(p)
	}

	ctx.Source.Reply(text.Translatable("commands.list.players",
		text.Text(fmt.Sprint(len(players))),
		text.Text(fmt.Sprint(s.conf.MaxPlayers)),
		text.List(parts),
	))
}
