package app

import "time"

// fpsTracker is a rolling frames-in-the-last-second counter — the direct,
// intuitive definition of "fps" (not a fixed-window average from program
// start), so it reflects current rendering health rather than a lifetime
// number that never recovers from an early stall.
type fpsTracker struct {
	times []time.Time
}

func (f *fpsTracker) record(now time.Time) {
	f.times = append(f.times, now)
	cutoff := now.Add(-time.Second)
	i := 0
	for i < len(f.times) && f.times[i].Before(cutoff) {
		i++
	}
	f.times = f.times[i:]
}

func (f *fpsTracker) fps() int {
	return len(f.times)
}
