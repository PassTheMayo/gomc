package server

import (
	"math/rand/v2"

	"main/src/block"
	"main/src/item"
	"main/src/loot"
	"main/src/packet"
)

// dropBlockLoot rolls a block's loot table and drops the items from it. tool
// is what broke it, empty for a block destroyed along with another, and
// byEntity reports whether a player broke it.
func (s *Server) dropBlockLoot(pos packet.BlockPos, state block.StateID, tool item.Stack, byEntity bool) {
	ctx := &loot.Context{
		State:    state,
		Tool:     tool,
		ByEntity: byEntity,
		BlockAt: func(dx, dy, dz int) block.StateID {
			s, _ := s.world.Block(int(pos.X)+dx, int(pos.Y)+dy, int(pos.Z)+dz)

			return s
		},
		Rand: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}

	for _, stack := range loot.BlockDrops(ctx) {
		s.dropFromBlock(pos, stack)
	}
}

// harvests reports whether breaking a block with a tool drops its loot:
// blocks like stone and ores need a tool whose rules mark it correct.
func harvests(tool item.Stack, state block.StateID) bool {
	return !block.RequiresCorrectTool(state) || !tool.IsEmpty() && item.PropertiesOf(tool.Item).CorrectForDrops(block.ByState(state).ID)
}
