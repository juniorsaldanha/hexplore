package views

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func sampleTxWithIO(vinCount, voutCount int) domain.Tx {
	t := domain.Tx{TxID: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456a", Fee: 45000, Weight: 400_000}
	for range vinCount {
		t.Vin = append(t.Vin, domain.Vin{Address: "bc1qexampleaddressxxxxxxxxxxxxxxxxxxxxxxxx", Value: 50000})
	}
	for range voutCount {
		t.Vout = append(t.Vout, domain.Vout{Address: "bc1qanotherexampleaddressxxxxxxxxxxxxxxxxx", Value: 49000})
	}
	return t
}

// TestTxRenderNeverExceedsReportedHeight is the regression test for a
// consolidation/CoinJoin tx (hundreds of inputs or outputs) blowing past the
// terminal height — the same class of bubbletea redraw-corruption bug fixed
// for the block view: View() output must not exceed the reported height.
func TestTxRenderNeverExceedsReportedHeight(t *testing.T) {
	cases := []struct {
		name                string
		width, height       int
		vinCount, voutCount int
		cursor              int
	}{
		{"huge consolidation tx, short terminal", 140, 24, 800, 1, 0},
		{"huge coinjoin tx, short terminal", 140, 24, 5, 400, 0},
		{"huge both sides, tall terminal", 140, 80, 500, 500, 0},
		{"cursor deep into a huge input list", 140, 24, 800, 1, 700},
		{"narrow terminal", 80, 24, 300, 300, 0},
		{"zero height (unknown, e.g. pre-WindowSizeMsg)", 140, 0, 800, 1, 0},
		{"small tx, nothing to scroll", 140, 24, 2, 2, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := TxData{Tx: sampleTxWithIO(c.vinCount, c.voutCount), Cursor: c.cursor, Width: c.width, Height: c.height}
			out := Tx(d, theme.Nord)
			got := lipgloss.Height(out)

			budget := c.height
			if budget <= 0 {
				budget = 40 // Tx()'s own fallback when height is unknown
			}
			if got > budget {
				t.Errorf("rendered %d lines, want <= %d\n%s", got, budget, out)
			}
		})
	}
}
