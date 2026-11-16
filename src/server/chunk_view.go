package server

import (
	"math"
	"slices"
	"sync"
	"time"

	"main/src/packet"
	"main/src/world"
)

const (
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

// inView reports whether pos is within radius of center, using the same
// rounded square as the vanilla server.
func inView(center world.ChunkPos, radius int, pos world.ChunkPos) bool {
	dx := max(0, abs32(pos.X-center.X)-1)
	dz := max(0, abs32(pos.Z-center.Z)-1)

	return int(dx*dx+dz*dz) < radius*radius
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}

	return v
}

func (v *chunkView) start() {
	v.mu.Lock()
	center := v.center
	v.mu.Unlock()

	v.player.send(&packet.SetChunkCacheCenter{X: center.X, Z: center.Z})

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
		v.player.send(&packet.SetChunkCacheCenter{X: center.X, Z: center.Z})
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
		if !inView(center, radius, pos) {
			v.player.send(&packet.ForgetLevelChunk{X: pos.X, Z: pos.Z})
			v.world.Release(pos)
			delete(v.sent, pos)
		}
	}

	if unacked >= maxUnackedBatches {
		return
	}

	var missing []world.ChunkPos
	r := int32(radius + 1)

	for x := center.X - r; x <= center.X+r; x++ {
		for z := center.Z - r; z <= center.Z+r; z++ {
			pos := world.ChunkPos{X: x, Z: z}

			if _, ok := v.sent[pos]; !ok && inView(center, radius, pos) {
				missing = append(missing, pos)
			}
		}
	}

	if len(missing) == 0 {
		return
	}

	distance := func(p world.ChunkPos) int32 {
		dx, dz := p.X-center.X, p.Z-center.Z
		return dx*dx + dz*dz
	}

	slices.SortFunc(missing, func(a, b world.ChunkPos) int { return int(distance(a) - distance(b)) })

	batch := missing[:min(len(missing), max(1, int(math.Ceil(float64(rate)))))]
	chunks := make([]*world.Chunk, len(batch))

	var wg sync.WaitGroup

	for i, pos := range batch {
		wg.Go(func() { chunks[i] = v.world.Acquire(pos) })
	}

	wg.Wait()

	v.player.send(&packet.ChunkBatchStart{})

	for i, c := range chunks {
		c.SendPacket(func(p *packet.LevelChunkWithLight) { v.player.send(p) })
		v.sent[batch[i]] = struct{}{}
	}

	v.player.send(&packet.ChunkBatchFinished{BatchSize: int32(len(chunks))})

	v.mu.Lock()
	v.unacked++
	v.mu.Unlock()
}
