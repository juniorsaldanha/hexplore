package app

import (
	"testing"
	"time"
)

func TestFPSTrackerCountsFramesInLastSecond(t *testing.T) {
	var f fpsTracker
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 60 frames spaced ~16.67ms apart, spanning just under 1s.
	for i := range 60 {
		f.record(base.Add(time.Duration(i) * (time.Second / 60)))
	}
	if got := f.fps(); got != 60 {
		t.Errorf("fps() = %d, want 60", got)
	}
}

func TestFPSTrackerPrunesStaleFrames(t *testing.T) {
	var f fpsTracker
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 10 frames a long time ago...
	for i := range 10 {
		f.record(base.Add(time.Duration(i) * time.Millisecond))
	}
	// ...then a gap of 2s, then 5 fresh frames.
	later := base.Add(2 * time.Second)
	for i := range 5 {
		f.record(later.Add(time.Duration(i) * time.Millisecond))
	}
	if got := f.fps(); got != 5 {
		t.Errorf("fps() = %d, want 5 (the old frames should have aged out)", got)
	}
}

func TestFPSTrackerEmpty(t *testing.T) {
	var f fpsTracker
	if got := f.fps(); got != 0 {
		t.Errorf("fps() on an empty tracker = %d, want 0", got)
	}
}

func TestFPSTrackerWindowBoundary(t *testing.T) {
	var f fpsTracker
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f.record(base)
	// Exactly 1s later, the first frame is still within an inclusive
	// "last second" window (cutoff = now-1s; a frame *at* the cutoff is
	// not "before" it, so it's kept).
	f.record(base.Add(time.Second))
	if got := f.fps(); got != 2 {
		t.Errorf("fps() = %d, want 2 (a frame exactly 1s old is still in-window)", got)
	}
	// A frame just past the boundary does age it out.
	f.record(base.Add(time.Second + time.Millisecond))
	if got := f.fps(); got != 2 {
		t.Errorf("fps() = %d, want 2 (the frame at t=0 should now be pruned)", got)
	}
}
