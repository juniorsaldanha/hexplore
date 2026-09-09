package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOpeningABlockTriggersTreemapFetch(t *testing.T) {
	m, _ := newTestModel()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.stack) != 1 || m.stack[0].kind != screenBlock {
		t.Fatalf("expected a block screen pushed, stack=%v", m.stack)
	}

	// The block-load cmd, once drained, must itself yield a follow-up
	// command (fetchBlockTreemap) — drain follows that single-command
	// chain (not a batch, so the "drain only the first batch element"
	// convention doesn't apply here).
	drain(t, m, cmd)

	if m.stack[0].blockTreemapLoading {
		t.Fatal("expected blockTreemapLoading to be cleared once the fetch resolved")
	}
	if len(m.stack[0].blockTreemapTxs) == 0 {
		t.Fatal("expected the treemap fetch to have populated blockTreemapTxs")
	}
	if out := m.View(); out == "" {
		t.Fatal("block view with treemap rendered empty")
	}
}
