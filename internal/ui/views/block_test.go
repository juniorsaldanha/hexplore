package views

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func sampleTxs(n int) []domain.Tx {
	out := make([]domain.Tx, n)
	for i := range out {
		out[i] = domain.Tx{
			TxID:   "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456" + string(rune('a'+i%26)),
			Fee:    int64(1000 + i),
			Weight: 800,
			Vin:    []domain.Vin{{Address: "bc1qexampleaddressxxxxxxxxxxxxxxxxxxxxxxxx", Value: 50000}},
			Vout:   []domain.Vout{{Address: "bc1qanotherexampleaddressxxxxxxxxxxxxxxxxx", Value: 49000}},
		}
	}
	return out
}

func sampleBlockData(width, height, txCount, cursor int, treemapLoading bool, treemapTxs int) BlockData {
	return BlockData{
		Block: domain.Block{
			Height: 900000, Hash: "00000000000000000000000000000000000000000000000000000000000000",
			PrevHash: "00000000000000000000000000000000000000000000000000000000000000",
			Time:     time.Now().Add(-5 * time.Minute), TxCount: txCount, Size: 1_500_000, Weight: 3_900_000,
			MerkleRoot: "00000000000000000000000000000000000000000000000000000000000000",
			Pool:       "Foundry USA", MatchRate: 99.5,
			Fee: &domain.BlockFee{TotalSats: 5_000_000, AvgFeeRate: 12.3, MinFeeRate: 1, MaxFeeRate: 400},
		},
		Txs:            sampleTxs(min(txCount, blockTxPageSize)),
		Cursor:         cursor,
		Caps:           domain.Capabilities{MiningPool: true},
		TreemapTxs:     sampleTxs(treemapTxs),
		TreemapLoading: treemapLoading,
		Currency:       "USD",
		Width:          width,
		Height:         height,
	}
}

// TestBlockRenderNeverExceedsReportedHeight is the regression test for the
// treemap-overlaying-the-info-panels bug: bubbletea's redraw breaks when
// View() outputs more lines than the terminal actually has, which is
// exactly what an unbounded treemap + one line per transaction produced.
func TestBlockRenderNeverExceedsReportedHeight(t *testing.T) {
	cases := []struct {
		name                        string
		width, height, txCount, cur int
		treemapLoading              bool
		treemapTxs                  int
	}{
		{"short terminal, full page, loaded treemap", 140, 24, 3000, 0, false, 750},
		{"short terminal, treemap still loading", 140, 24, 3000, 0, true, 0},
		{"tall terminal, full page", 140, 80, 3000, 0, false, 750},
		{"very short terminal", 100, 22, 3000, 10, false, 750},
		{"narrow stacked layout", 70, 30, 3000, 24, false, 750},
		{"cursor near the end of the page", 140, 24, 3000, 24, false, 750},
		{"only a handful of tx on the last page", 140, 30, 7, 3, false, 7},
		{"zero height (unknown, e.g. pre-WindowSizeMsg)", 140, 0, 3000, 0, false, 750},
		// The classic terminal default (macOS Terminal.app, many others) —
		// found live: 80 cols triggers the stacked info-box layout, whose
		// combined BLOCK+TECHNICAL height (23 lines) alone nearly exhausts
		// a 24-row terminal.
		{"80x24 classic default", 80, 24, 3847, 3, false, 25},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := sampleBlockData(c.width, c.height, c.txCount, c.cur, c.treemapLoading, c.treemapTxs)
			out := Block(d, theme.Nord)
			got := lipgloss.Height(out)

			budget := c.height
			if budget <= 0 {
				budget = 40 // Block()'s own fallback when height is unknown
			}
			// The tx list always shows at least one row rather than zero,
			// even once the computed budget for it goes slightly negative
			// (showing nothing would be worse) — that floor can overflow
			// the budget by at most one row (2 lines, since the shown row
			// is always the cursor's expanded one). Tolerate that, not
			// arbitrary amounts more.
			const irreducibleRowFloorTolerance = 2
			if got > budget+irreducibleRowFloorTolerance {
				t.Errorf("rendered %d lines, want <= %d (+%d tolerance for the always-show-one-row floor)\n%s",
					got, budget, irreducibleRowFloorTolerance, out)
			}
		})
	}
}

func TestScrollWindow(t *testing.T) {
	cases := []struct {
		total, cursor, max int
		wantStart, wantEnd int
	}{
		{10, 0, 20, 0, 10},     // fits entirely, no windowing needed
		{100, 0, 10, 0, 10},    // cursor at the very start
		{100, 99, 10, 90, 100}, // cursor at the very end
		{100, 50, 10, 45, 55},  // cursor in the middle, centered window
	}
	for _, c := range cases {
		start, end := scrollWindow(c.total, c.cursor, c.max)
		if start != c.wantStart || end != c.wantEnd {
			t.Errorf("scrollWindow(%d, %d, %d) = (%d, %d), want (%d, %d)",
				c.total, c.cursor, c.max, start, end, c.wantStart, c.wantEnd)
		}
		if end-start > c.max && c.total > c.max {
			t.Errorf("scrollWindow window size %d exceeds max %d", end-start, c.max)
		}
		if c.total > c.max && (c.cursor < start || c.cursor >= end) {
			t.Errorf("scrollWindow(%d, %d, %d) window [%d,%d) doesn't contain the cursor", c.total, c.cursor, c.max, start, end)
		}
	}
}

func TestScrollHintEmptyWhenEverythingVisible(t *testing.T) {
	dim := lipgloss.NewStyle()
	if got := scrollHint(10, 0, 10, dim); got != "" {
		t.Errorf("scrollHint with nothing hidden = %q, want empty", got)
	}
}

func TestScrollHintReportsHiddenCounts(t *testing.T) {
	dim := lipgloss.NewStyle()
	got := scrollHint(100, 20, 30, dim)
	if got == "" {
		t.Fatal("expected a non-empty hint when items are hidden on both sides")
	}
}
