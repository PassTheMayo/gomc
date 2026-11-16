package server

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"main/src/command"
	"main/src/packet"
	"main/src/text"
	"main/src/uuid"
	"main/src/world"
)

const (
	// maxPregenRadius bounds /pregen to a square of 2049x2049 chunks.
	maxPregenRadius = 1024

	// pregenProgressInterval is how often the player who started a
	// pre-generation sees its progress.
	pregenProgressInterval = 2 * time.Second
)

// pregenJob generates every chunk in a square around a center chunk.
type pregenJob struct {
	owner      uuid.UUID
	center     world.ChunkPos
	radius     int
	total      int64
	started    time.Time
	done       atomic.Int64
	generated  atomic.Int64
	failed     atomic.Int64
	cancel     chan struct{}
	cancelOnce sync.Once
}

func (j *pregenJob) stop() {
	j.cancelOnce.Do(func() { close(j.cancel) })
}

// registerPregen adds /pregen, which generates chunks ahead of time.
func (s *Server) registerPregen() {
	s.commands.Register(command.Literal("pregen",
		command.Integer("radius", 1, maxPregenRadius).Executes(s.startPregen),
		command.Literal("cancel").Executes(s.cancelPregen),
	), true)
}

// startPregen handles /pregen <radius>: it generates every chunk within
// radius chunks of the player's chunk, measured as a square.
func (s *Server) startPregen(ctx *command.Context) {
	p := ctx.Source.(*Player)
	radius := int(ctx.Int("radius"))
	side := int64(2*radius + 1)
	job := &pregenJob{
		owner:   p.Profile.ID,
		center:  p.snapshot().chunkPos(),
		radius:  radius,
		total:   side * side,
		started: time.Now(),
		cancel:  make(chan struct{}),
	}

	s.pregenMu.Lock()

	if s.pregen != nil {
		s.pregenMu.Unlock()
		p.Reply(text.Text("A pre-generation is already running. Stop it with /pregen cancel.").Colored(text.Red))

		return
	}

	s.pregen = job
	s.pregenMu.Unlock()

	p.Reply(text.Text(fmt.Sprintf("Pre-generating %d chunks within %d chunks of chunk %d, %d.",
		job.total, radius, job.center.X, job.center.Z)))
	s.log.Info("Started chunk pre-generation", "player", p.Profile.Name, "center_x", job.center.X,
		"center_z", job.center.Z, "radius", radius, "chunks", job.total)

	s.wg.Add(1)
	go s.pregenerate(job)
}

// cancelPregen handles /pregen cancel.
func (s *Server) cancelPregen(ctx *command.Context) {
	s.pregenMu.Lock()
	job := s.pregen
	s.pregenMu.Unlock()

	if job == nil {
		ctx.Source.Reply(text.Text("No pre-generation is running.").Colored(text.Red))
		return
	}

	job.stop()
	ctx.Source.Reply(text.Text("Stopping the pre-generation."))
}

// pregenerate runs a job. Chunks are generated in rings around the center so
// the nearest are ready first, and one CPU is left for players' own chunks.
func (s *Server) pregenerate(job *pregenJob) {
	defer s.wg.Done()

	positions := make(chan world.ChunkPos)
	var workers sync.WaitGroup
	var firstErr sync.Once

	for range max(1, runtime.GOMAXPROCS(0)-1) {
		workers.Go(func() {
			for pos := range positions {
				generated, err := s.world.Pregenerate(pos)

				switch {
				case err != nil:
					job.failed.Add(1)
					firstErr.Do(func() { s.log.Error("Failed to save a pre-generated chunk", "x", pos.X, "z", pos.Z, "error", err) })
				case generated:
					job.generated.Add(1)
				}

				job.done.Add(1)
			}
		})
	}

	finished := make(chan struct{})

	go func() {
		ticker := time.NewTicker(pregenProgressInterval)
		defer ticker.Stop()

		for {
			select {
			case <-finished:
				return
			case <-ticker.C:
				s.notifyPregenOwner(job, &packet.SystemChat{Content: job.progress(), Overlay: true})
			}
		}
	}()

	stopped := false

	for r := int32(0); r <= int32(job.radius) && !stopped; r++ {
		for _, pos := range world.Ring(job.center, r) {
			select {
			case positions <- pos:
				continue
			case <-job.cancel:
			case <-s.quit:
			}

			stopped = true

			break
		}
	}

	close(positions)
	workers.Wait()
	close(finished)

	s.pregenMu.Lock()
	s.pregen = nil
	s.pregenMu.Unlock()

	elapsed := time.Since(job.started).Round(time.Second)
	done, generated, failed := job.done.Load(), job.generated.Load(), job.failed.Load()

	var msg text.Component

	if stopped {
		msg = text.Text(fmt.Sprintf("Pre-generation stopped after %d of %d chunks (%d new) in %s.", done, job.total, generated, elapsed)).Colored(text.Yellow)
	} else {
		msg = text.Text(fmt.Sprintf("Pre-generated %d chunks (%d new) in %s.", done, generated, elapsed)).Colored(text.Green)
	}

	if failed > 0 {
		msg = msg.Append(text.Text(fmt.Sprintf(" %d chunks failed to save, see the server log.", failed)).Colored(text.Red))
	}

	s.notifyPregenOwner(job, &packet.SystemChat{Content: msg})
	s.log.Info("Finished chunk pre-generation", "chunks", done, "generated", generated, "failed", failed,
		"stopped", stopped, "elapsed", elapsed.String())
}

// progress describes how far the job is, for the action bar.
func (j *pregenJob) progress() text.Component {
	done := j.done.Load()
	elapsed := time.Since(j.started).Seconds()
	rate := float64(done) / max(elapsed, 0.001)
	msg := fmt.Sprintf("Pre-generating: %d / %d chunks (%d%%), %.0f chunks/s", done, j.total, done*100/j.total, rate)

	if rate > 0 {
		left := time.Duration(float64(j.total-done)/rate) * time.Second
		msg += fmt.Sprintf(", %s left", left.Round(time.Second))
	}

	return text.Text(msg)
}

// notifyPregenOwner sends a message to the player who started the job, if
// they are still online.
func (s *Server) notifyPregenOwner(job *pregenJob, msg packet.Packet) {
	s.mu.RLock()
	owner := s.players[job.owner]
	s.mu.RUnlock()

	if owner != nil && owner.joined.Load() {
		owner.Send(msg)
	}
}
