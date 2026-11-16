package server

import (
	"main/src/command"
	"main/src/packet"
	"main/src/text"
)

// registerGameModeCommand adds /gamemode <gamemode> [<target>], which
// operators use to change their own or another player's game mode.
func (s *Server) registerGameModeCommand() {
	s.commands.Register(command.Literal("gamemode",
		command.GameMode("gamemode",
			command.Players("target").Executes(func(ctx *command.Context) {
				target := s.playerByName(ctx.String("target"))

				if target == nil {
					ctx.Source.Reply(text.Translatable("argument.entity.notfound.player").Colored(text.Red))
					return
				}

				s.changeGameMode(ctx, target)
			}),
		).Executes(func(ctx *command.Context) {
			p, ok := ctx.Source.(*Player)

			if !ok {
				ctx.Source.Reply(text.Translatable("permissions.requires.player").Colored(text.Red))
				return
			}

			s.changeGameMode(ctx, p)
		}),
	), true)
}

// changeGameMode sets the target's game mode from the command's argument and
// tells the source and the target. Like vanilla, nothing is shown when the
// target is already in that game mode.
func (s *Server) changeGameMode(ctx *command.Context, target *Player) {
	mode := ctx.GameMode("gamemode")

	if !s.setGameMode(target, mode) {
		return
	}

	name := text.Translatable("gameMode." + packet.GameModeNames[mode])

	if ctx.Source == target {
		ctx.Source.Reply(text.Translatable("commands.gamemode.success.self", name))
		return
	}

	target.Reply(text.Translatable("gameMode.changed", name))
	ctx.Source.Reply(text.Translatable("commands.gamemode.success.other", text.Text(target.Profile.Name), name))
}
