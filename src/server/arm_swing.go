package server

import (
	"main/src/item"
	"main/src/packet"
)

// registerArmSwing shows other players the arm swings of players who punch,
// which clients send while attacking and mining.
func (s *Server) registerArmSwing() {
	on(&s.router, func(p *Player, _ *packet.Punch) {
		p.mu.Lock()
		animation := p.inventory.MainHand().AttackAnimation()
		p.mu.Unlock()

		s.swingArm(p, packet.HandMain, animation)
	})
}

// swingArm shows other players p swinging an arm. The player's own client
// already shows the swings it predicts.
func (s *Server) swingArm(p *Player, hand int32, animation item.SwingAnimation) {
	p.mu.Lock()
	started := p.swing.Start(s.gameTime.Load(), animation.Duration)
	p.mu.Unlock()

	if started {
		s.entities.SendToViewers(p.EntityID, &packet.SwingAnimation{EntityID: p.EntityID, Hand: hand, Animation: animation})
	}
}
