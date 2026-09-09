package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFrameTickDrivesFPSTracker is the integration check tying the fps
// counter to real Update() dispatch — not just the pure fpsTracker math
// (fps_test.go) or the pure Dashboard render cost (views/perf_test.go),
// but that a frameTickMsg flowing through Model.Update actually records a
// frame and re-arms the next tick, the same way Init()'s frameTick() cmd
// drives it during a real run.
func TestFrameTickDrivesFPSTracker(t *testing.T) {
	m, _ := newTestModel()
	if got := m.fps.fps(); got != 0 {
		t.Fatalf("fps before any tick = %d, want 0", got)
	}

	now := time.Now()
	var cmd tea.Cmd
	for i := range 10 {
		var teaModel tea.Model
		teaModel, cmd = m.Update(frameTickMsg(now.Add(time.Duration(i) * (time.Second / 60))))
		m = teaModel.(*Model)
		if cmd == nil {
			t.Fatalf("tick %d: expected Update to re-arm another frameTick", i)
		}
	}
	if got := m.fps.fps(); got != 10 {
		t.Fatalf("fps after 10 ticks within 1s = %d, want 10", got)
	}
}

func TestFrameTickCmdFiresAtTargetInterval(t *testing.T) {
	before := time.Now()
	cmd := frameTick()
	msg := cmd() // frameTick's tea.Tick blocks for frameInterval, then fires
	elapsed := time.Since(before)

	tm, ok := msg.(frameTickMsg)
	if !ok {
		t.Fatalf("frameTick() produced %T, want frameTickMsg", msg)
	}
	if elapsed < frameInterval/2 {
		t.Errorf("frameTick fired after only %v, expected to wait ~%v", elapsed, frameInterval)
	}
	if time.Time(tm).IsZero() {
		t.Error("frameTickMsg carries a zero time")
	}
}
