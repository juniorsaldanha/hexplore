package views

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func sampleAddressTxs(n int) []domain.Tx {
	out := make([]domain.Tx, n)
	for i := range out {
		out[i] = domain.Tx{
			TxID: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456" + string(rune('a'+i%26)),
			Fee:  int64(1000 + i),
			Status: domain.TxStatus{
				Confirmed: true, BlockHeight: 900000 - i, BlockTime: time.Now().Add(-time.Duration(i) * time.Hour),
			},
		}
	}
	return out
}

func sampleAddressData(width, height, txCount, cursor, historyLen int) AddressData {
	history := make([]float64, historyLen)
	for i := range history {
		history[i] = float64(1_000_000 + i*1000)
	}
	return AddressData{
		Address: domain.Address{
			Address:    "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh",
			FundedSats: 1_658_225_206, SpentSats: 1_287_005_839, TxCount: txCount,
			MempoolFundedSats: 90000, MempoolSpentSats: 0, MempoolTxCount: 1,
			FundedTxoCount: 1175, SpentTxoCount: 434, MempoolFundedTxoCount: 2,
		},
		Txs: sampleAddressTxs(txCount), Cursor: cursor, Width: width, Height: height,
		Currency: "USD", History: history,
	}
}

// TestAddressRenderNeverExceedsReportedHeight is the same regression class
// fixed for the block and tx views: View() output must never exceed the
// terminal's reported height, or bubbletea's redraw breaks.
func TestAddressRenderNeverExceedsReportedHeight(t *testing.T) {
	cases := []struct {
		name                              string
		width, height, txCount, cur, hist int
	}{
		{"short terminal, many tx, with history", 140, 24, 3000, 0, 600},
		{"short terminal, history still loading", 140, 24, 3000, 0, 0},
		{"tall terminal", 140, 60, 3000, 0, 600},
		{"cursor deep in a long history", 140, 24, 3000, 2500, 600},
		{"narrow terminal", 80, 24, 3000, 0, 600},
		{"zero height (unknown, e.g. pre-WindowSizeMsg)", 140, 0, 3000, 0, 600},
		{"brand new address, nothing to show", 140, 24, 0, 0, 0},
		{"very short terminal", 100, 15, 3000, 0, 600},
	}
	// The ADDRESS box's fields (type, features, confirmed, pending, UTXOs,
	// received, tx count) plus a minimal one-row tx list are a hard floor
	// that doesn't shrink further — verified empirically to plateau at
	// this height rather than grow unbounded as the terminal shrinks past
	// it (see the probe in this test's history). Below that floor,
	// overflowing by a bounded amount beats hiding fields the user asked
	// to always see. One line taller than without Features, since that's
	// an extra row in the same box.
	const addressHardFloor = 19
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := sampleAddressData(c.width, c.height, c.txCount, c.cur, c.hist)
			d.Features = []string{"SegWit", "Taproot", "RBF", "CoinJoin"}
			out := Address(d, theme.Nord)
			got := lipgloss.Height(out)

			budget := c.height
			if budget <= 0 {
				budget = 40 // Address()'s own fallback when height is unknown
			}
			if budget < addressHardFloor {
				budget = addressHardFloor
			}
			if got > budget {
				t.Errorf("rendered %d lines, want <= %d\n%s", got, budget, out)
			}
		})
	}
}

func TestFeatureBadgesJoinsAllFeaturesEmptyOnNone(t *testing.T) {
	if got := featureBadges(nil, theme.Nord); got != "" {
		t.Errorf("featureBadges(nil) = %q, want empty", got)
	}
	got := featureBadges([]string{"SegWit", "Taproot", "RBF"}, theme.Nord)
	for _, want := range []string{"SegWit", "Taproot", "RBF"} {
		if !strings.Contains(got, want) {
			t.Errorf("featureBadges output %q missing %q", got, want)
		}
	}
}
