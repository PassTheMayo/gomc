package server

import "time"

// throttle limits how often something happens, like vanilla's TickThrottler:
// each use adds cost to a level that drains by one every tick, and uses are
// refused while the level is at limit.
type throttle struct {
	cost, limit float64
	level       float64
	last        time.Time
}

// allow reports whether a use is allowed now and counts it if so.
func (t *throttle) allow(now time.Time) bool {
	if !t.last.IsZero() {
		t.level = max(0, t.level-now.Sub(t.last).Seconds()/tickDuration.Seconds())
	}

	t.last = now

	if t.level >= t.limit {
		return false
	}

	t.level += t.cost

	return true
}
