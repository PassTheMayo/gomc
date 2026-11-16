package server

import "main/src/packet"

// registerCommands runs chat commands and sends joining players the command
// tree they may use. Features add their commands to s.commands.
func (s *Server) registerCommands() {
	s.router.onJoin(func(p *Player) { p.Send(s.commands.Tree(p)) })

	on(&s.router, func(p *Player, pk *packet.ChatCommand) {
		p.session.log.Info("Player ran a command", "command", pk.Command)
		s.commands.Execute(p, pk.Command)
	})
}
