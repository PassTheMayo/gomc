package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// scheduler runs features' periodic tasks on the game loop, in registration
// order. Tasks are added while the server is created, before the loop starts.
type scheduler struct {
	tasks []scheduledTask
}

type scheduledTask struct {
	interval int64
	run      func(t *tick)
}

// tick is the state shared by every task in one game tick.
type tick struct {
	number  int64
	now     time.Time
	players []*Player
}

// every runs a task on the game loop every interval ticks.
func (s *scheduler) every(interval int64, run func(t *tick)) {
	s.tasks = append(s.tasks, scheduledTask{interval: interval, run: run})
}

// everyAsync runs a task in the background every interval ticks, for work
// that can block, such as disk access. A run is skipped while the previous
// one is still going. wg tracks the runs so shutdown can wait for them.
func (s *scheduler) everyAsync(interval int64, wg *sync.WaitGroup, run func(t *tick)) {
	var running atomic.Bool

	s.every(interval, func(t *tick) {
		if running.CompareAndSwap(false, true) {
			wg.Go(func() {
				defer running.Store(false)
				run(t)
			})
		}
	})
}

// run runs the tasks due in a tick.
func (s *scheduler) run(t *tick) {
	for _, task := range s.tasks {
		if t.number%task.interval == 0 {
			task.run(t)
		}
	}
}
