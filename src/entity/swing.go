package entity

// Swing is an entity's arm swing. Like vanilla's SwingState, a new swing only
// starts in the tick the current one began or once it is past halfway, so
// holding the attack key doesn't restart the animation every tick.
type Swing struct {
	started  bool
	start    int64
	duration int32
}

// Start begins a swing lasting duration ticks at game tick now, if able, and
// reports whether it started.
func (s *Swing) Start(now int64, duration int32) bool {
	if elapsed := now - s.start; s.started && elapsed > 0 && elapsed <= int64(s.duration/2) {
		return false
	}

	s.started, s.start, s.duration = true, now, duration

	return true
}
