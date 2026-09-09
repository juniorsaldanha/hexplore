package app

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/ui/views"
)

func TestOpenLiveBlockFetchesAndRenders(t *testing.T) {
	m, fc := newTestModel()
	fc.liveBlockTxs = true
	fc.projectedTxs = []domain.ProjectedTx{
		{TxID: "a", VSize: 200, FeeRate: 10},
		{TxID: "b", VSize: 300, FeeRate: 60, Flags: 1 << 33}, // consolidation
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if len(m.stack) != 1 || m.stack[0].kind != screenLiveBlock {
		t.Fatalf("expected live block screen pushed, stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if len(m.stack[0].liveBlockTxs) != 2 {
		t.Fatalf("expected 2 projected txs loaded, got %d", len(m.stack[0].liveBlockTxs))
	}
	if out := m.View(); out == "" {
		t.Fatal("live block view rendered empty")
	}
}

func TestLiveBlockFilterCycles(t *testing.T) {
	m, fc := newTestModel()
	fc.liveBlockTxs = true
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "a", VSize: 200, FeeRate: 10}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	drain(t, m, cmd)

	if m.stack[0].liveBlockFilter != views.FilterAll {
		t.Fatalf("initial filter = %v, want FilterAll", m.stack[0].liveBlockFilter)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if m.stack[0].liveBlockFilter != views.FilterConsolidation {
		t.Fatalf("filter after one cycle = %v, want FilterConsolidation", m.stack[0].liveBlockFilter)
	}
}

func TestOpenLiveBlockUnsupportedProviderNoFetch(t *testing.T) {
	m, fc := newTestModel()
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "should-not-be-used"}}

	// fakeChain.Caps() doesn't set LiveBlockTxs, so it defaults false.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if len(m.stack) != 1 || m.stack[0].kind != screenLiveBlock {
		t.Fatalf("expected live block screen pushed even when unsupported, stack=%v", m.stack)
	}
	if cmd != nil {
		t.Fatal("expected no fetch command for an unsupported provider")
	}
	out := m.View()
	if out == "" {
		t.Fatal("unsupported live block view rendered empty")
	}
}

func TestLiveBlockTickRefreshesWhileOpenAndReArms(t *testing.T) {
	m, fc := newTestModel()
	fc.liveBlockTxs = true
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "a", VSize: 100}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	drain(t, m, cmd) // initial fetch only (drain skips the batched tick)

	// Simulate the tick firing, without ever invoking the real timer.
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "b", VSize: 200}, {TxID: "c", VSize: 300}}
	teaModel, tickCmd := m.Update(liveBlockTickMsg{})
	m = teaModel.(*Model)
	if tickCmd == nil {
		t.Fatal("expected the tick to re-arm with another batch(fetch, tick)")
	}
	drain(t, m, tickCmd)
	if len(m.stack[0].liveBlockTxs) != 2 {
		t.Fatalf("expected the refreshed data (2 txs), got %d", len(m.stack[0].liveBlockTxs))
	}
}

func TestLiveBlockTickStopsAfterNavigatingAway(t *testing.T) {
	m, fc := newTestModel()
	fc.liveBlockTxs = true
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "a", VSize: 100}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	drain(t, m, cmd)

	m.pop() // back to the dashboard
	_, tickCmd := m.Update(liveBlockTickMsg{})
	if tickCmd != nil {
		t.Fatal("expected the tick chain to stop once the live-block screen is no longer on top")
	}
}

func TestLiveBlockRefreshErrorKeepsLastGoodData(t *testing.T) {
	m, fc := newTestModel()
	fc.liveBlockTxs = true
	fc.projectedTxs = []domain.ProjectedTx{{TxID: "a", VSize: 100}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	drain(t, m, cmd)
	if len(m.stack[0].liveBlockTxs) != 1 {
		t.Fatalf("expected initial data loaded, got %d txs", len(m.stack[0].liveBlockTxs))
	}

	fc.projectedErr = errors.New("transient network error")
	teaModel, tickCmd := m.Update(liveBlockTickMsg{})
	m = teaModel.(*Model)
	drain(t, m, tickCmd)

	if len(m.stack[0].liveBlockTxs) != 1 {
		t.Fatalf("expected the previous good data to survive a refresh error, got %d txs", len(m.stack[0].liveBlockTxs))
	}
	if m.stack[0].err != nil {
		t.Fatalf("expected no error surfaced while stale-but-good data is on screen, got %v", m.stack[0].err)
	}
}

func TestClassifyFlagsUsedConsistentlyWithLiveBlock(t *testing.T) {
	// Sanity check that the flag this test file hardcodes above (1<<33)
	// really does mean "consolidation" per enrich's decoding.
	if enrich.ClassifyFlags(1<<33) != enrich.TxConsolidation {
		t.Fatal("test fixture's flag no longer decodes to TxConsolidation — enrich's bits changed?")
	}
}
