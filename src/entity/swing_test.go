package entity

import "testing"

func TestSwing(t *testing.T) {
	var s Swing

	steps := []struct {
		now  int64
		want bool
	}{
		{10, true},  // nothing is swinging
		{10, true},  // the same tick
		{11, false}, // the first half of a six tick swing
		{13, false},
		{14, true}, // past halfway
		{100, true},
	}

	for _, step := range steps {
		if got := s.Start(step.now, 6); got != step.want {
			t.Fatalf("Start at tick %d = %v, want %v", step.now, got, step.want)
		}
	}
}
