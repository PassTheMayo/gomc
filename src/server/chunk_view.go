package server

import (
	"math"
	"sync"
	"time"

	"main/src/packet"
	"main/src/world"
)

const (
	// unloadTicks is how often idle chunks are unloaded.
	unloadTicks = 600

	// chunkIdleTime is how long a chunk stays loaded without viewers.
	chunkIdleTime = 30 * time.Second

	// maxUnackedBatches is how many chunk batches may be in flight before the
	// server waits for the client to acknowledge them.
	maxUnackedBatches = 10

	// initialChunksPerTick is the batch size used until the client reports
	// how many chunks it can handle per tick.
	initialChunksPerTick = 9

	// maxChunksPerBatch bounds a single batch.
	maxChunksPerBatch = 64
)

// chunkView streams the chunks around a player. It runs in its own goroutine
// so that loading and generating chunks never stalls packet handling.
type chunkView struct {
	player  *Player
	world   *world.World
	wake    chan struct{}
	done    chan struct{}
	stopped chan struct{}

	mu      sync.Mutex
	center  world.ChunkPos
	radius  int
	unacked int
	rate    float32

	// Owned by the streaming goroutine.
	sent map[world.ChunkPos]struct{}
}

// registerChunkStreaming paces chunk batches by the client's acknowledgements
// and unloads chunks nobody has viewed for a while.
func (s *Server) registerChunkStreaming() {
	on(&s.router, func(p *Player, pk *packet.ChunkBatchReceived) { p.chunks.acknowledge(pk.DesiredChunksPerTick) })
	s.ticks.everyAsync(unloadTicks, &s.wg, func(*tick) { s.world.UnloadIdle(chunkIdleTime) })
}

func newChunkView(p *Player, w *world.World, center world.ChunkPos, radius int) *chunkView {
	return &chunkView{
		player:  p,
		world:   w,
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		center:  center,
		radius:  radius,
		rate:    initialChunksPerTick,
		sent:    make(map[world.ChunkPos]struct{}),
	}
}

func (v *chunkView) start() {
	v.mu.Lock()
	center := v.center
	v.mu.Unlock()

	v.player.Send(&packet.SetChunkCacheCenter{X: center.X, Z: center.Z})

	go v.run()
}

// stop ends streaming and releases every chunk the player held.
func (v *chunkView) stop() {
	close(v.done)
	<-v.stopped

	for pos := range v.sent {
		v.world.Release(pos)
	}
}

func (v *chunkView) notify() {
	select {
	case v.wake <- struct{}{}:
	default:
	}
}

// moveTo recenters the view on the chunk the player moved into.
func (v *chunkView) moveTo(center world.ChunkPos) {
	v.mu.Lock()
	changed := v.center != center
	v.center = center
	v.mu.Unlock()

	if changed {
		v.player.Send(&packet.SetChunkCacheCenter{X: center.X, Z: center.Z})
		v.notify()
	}
}

func (v *chunkView) setRadius(radius int) {
	v.mu.Lock()
	v.radius = radius
	v.mu.Unlock()

	v.notify()
}

// acknowledge records an acknowledged batch and the client's desired rate.
func (v *chunkView) acknowledge(chunksPerTick float32) {
	v.mu.Lock()
	v.unacked = max(0, v.unacked-1)

	if !math.IsNaN(float64(chunksPerTick)) && chunksPerTick > 0 {
		v.rate = min(chunksPerTick, maxChunksPerBatch)
	}

	v.mu.Unlock()

	v.notify()
}

// run sends at most one batch per game tick, like the vanilla server.
func (v *chunkView) run() {
	defer close(v.stopped)

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		v.update()

		select {
		case <-v.done:
			return
		case <-v.wake:
		case <-ticker.C:
		}
	}
}

func (v *chunkView) update() {
	v.mu.Lock()
	center, radius, unacked, rate := v.center, v.radius, v.unacked, v.rate
	v.mu.Unlock()

	for pos := range v.sent {
		if !world.InView(center, radius, pos) {
			v.player.Send(&packet.ForgetLevelChunk{X: pos.X, Z: pos.Z})
			v.world.Release(pos)
			delete(v.sent, pos)
		}
	}

	if unacked >= maxUnackedBatches {
		return
	}

	size := max(1, int(math.Ceil(float64(rate))))
	var batch []world.ChunkPos

	for pos := range world.InViewNearestFirst(center, radius) {
		if _, ok := v.sent[pos]; !ok {
			if batch = append(batch, pos); len(batch) == size {
				break
			}
		}
	}

	if len(batch) == 0 {
		return
	}
	chunks := make([]*world.Chunk, len(batch))

	var wg sync.WaitGroup

	for i, pos := range batch {
		wg.Go(func() { chunks[i] = v.world.Acquire(pos) })
	}

	wg.Wait()

	v.player.Send(&packet.ChunkBatchStart{})

	for i, c := range chunks {
		c.SendPacket(func(p *packet.LevelChunkWithLight) { v.player.Send(p) })
		v.sent[batch[i]] = struct{}{}
	}

	v.player.Send(&packet.ChunkBatchFinished{BatchSize: int32(len(chunks))})

	v.mu.Lock()
	v.unacked++
	v.mu.Unlock()
}
