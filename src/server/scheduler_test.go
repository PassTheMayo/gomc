package server

import (
	"slices"
	"sync"
	"testing"
)

func TestScheduler(t *testing.T) {
	var s scheduler
	var ran []int64

	s.every(1, func(t *tick) { ran = append(ran, t.number) })
	s.every(3, func(t *tick) { ran = append(ran, -t.number) })

	for n := int64(1); n <= 6; n++ {
		s.run(&tick{number: n})
	}

	if want := []int64{1, 2, 3, -3, 4, 5, 6, -6}; !slices.Equal(ran, want) {
		t.Fatalf("tasks ran as %v, want %v", ran, want)
	}
}

// A background task is skipped while its previous run is still going.
func TestSchedulerAsync(t *testing.T) {
	var s scheduler
	var wg sync.WaitGroup
	release := make(chan struct{})
	runs := 0

	s.everyAsync(1, &wg, func(*tick) {
		runs++
		<-release
	})

	for n := int64(1); n <= 3; n++ {
		s.run(&tick{number: n})
	}

	close(release)
	wg.Wait()

	if runs != 1 {
		t.Fatalf("the task ran %d times while busy, want 1", runs)
	}

	s.run(&tick{number: 4})
	wg.Wait()

	if runs != 2 {
		t.Fatalf("the task ran %d times after finishing, want 2", runs)
	}
}
