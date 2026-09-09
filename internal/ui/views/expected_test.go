package views

import (
	"strings"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func TestExpectedVsActualLineEmptyWithoutNativeData(t *testing.T) {
	d := BlockData{Block: domain.Block{Fee: &domain.BlockFee{TotalSats: 100}}}
	if got := expectedVsActualLine(d, theme.Nord); got != "" {
		t.Errorf("expected empty string without ExpectedFeeSats/ExpectedWeight, got %q", got)
	}
}

func TestExpectedVsActualLineWithNativeData(t *testing.T) {
	d := BlockData{
		Block: domain.Block{
			Weight:          4_000_000,
			ExpectedFeeSats: 1_000_000,
			ExpectedWeight:  3_900_000,
			Fee:             &domain.BlockFee{TotalSats: 950_000},
		},
		Currency: "USD",
	}
	got := expectedVsActualLine(d, theme.Nord)
	if got == "" {
		t.Fatal("expected a non-empty comparison line")
	}
	if !strings.Contains(got, "expected") || !strings.Contains(got, "actual") {
		t.Errorf("comparison line missing expected/actual labels: %q", got)
	}
}

func TestDeltaBadgeDirection(t *testing.T) {
	up := deltaBadge(110, 100, theme.Nord)  // actual > expected
	down := deltaBadge(90, 100, theme.Nord) // actual < expected
	if !strings.Contains(up, "▲") {
		t.Errorf("expected an up arrow when actual > expected, got %q", up)
	}
	if !strings.Contains(down, "▼") {
		t.Errorf("expected a down arrow when actual < expected, got %q", down)
	}
}

func TestDeltaBadgeZeroExpected(t *testing.T) {
	if got := deltaBadge(100, 0, theme.Nord); got != "" {
		t.Errorf("deltaBadge with expected=0 should avoid dividing by zero, got %q", got)
	}
}
