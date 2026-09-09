package components

import (
	"testing"
	"time"
	"unicode/utf8"
)

func TestFillBar(t *testing.T) {
	cases := []struct {
		pct   float64
		width int
	}{
		{0, 10}, {1, 10}, {0.5, 10}, {0.05, 10}, {-1, 10}, {2, 10}, {0.5, 0},
	}
	for _, c := range cases {
		got := FillBar(c.pct, c.width)
		if n := utf8.RuneCountInString(got); n != c.width {
			t.Errorf("FillBar(%v, %d) has %d runes, want %d", c.pct, c.width, n, c.width)
		}
	}
	if got := FillBar(1, 10); got != "██████████" {
		t.Errorf("FillBar(1, 10) = %q", got)
	}
}

func TestSparkline(t *testing.T) {
	series := []float64{1, 2, 3, 4, 5, 4, 3, 2, 1}
	got := Sparkline(series, 5)
	if n := utf8.RuneCountInString(got); n != 5 {
		t.Errorf("Sparkline width = %d, want 5", n)
	}
	if Sparkline(nil, 5) != "" {
		t.Error("Sparkline(nil, ...) should be empty")
	}
	if Sparkline([]float64{5, 5, 5}, 3) != "▁▁▁" {
		t.Errorf("flat series should render as the lowest level, got %q", Sparkline([]float64{5, 5, 5}, 3))
	}
}

func TestBigText(t *testing.T) {
	out := BigText("1")
	if len(out) != BigTextHeight {
		t.Fatalf("BigText returned %d rows, want %d", len(out), BigTextHeight)
	}
	for _, row := range out {
		if n := utf8.RuneCountInString(row); n != 3 {
			t.Errorf("single-glyph row width = %d, want 3", n)
		}
	}
}

func TestBigTextWidth(t *testing.T) {
	cases := []string{"", "1", "12", "1,234.56"}
	for _, s := range cases {
		got := BigTextWidth(s)
		want := utf8.RuneCountInString(BigText(s)[0])
		if got != want {
			t.Errorf("BigTextWidth(%q) = %d, want %d (actual rendered width)", s, got, want)
		}
	}
}

func TestBigTextUnknownRuneRendersBlankNotDropped(t *testing.T) {
	// An unsupported rune (e.g. a currency letter) must still occupy a
	// cell — dropping it would silently misalign anything rendered after.
	withUnknown := BigText("1X2")
	known := BigText("1 2")
	if withUnknown[0] != known[0] {
		t.Errorf("unknown rune should render as a blank cell like ' ': got %q, want %q", withUnknown[0], known[0])
	}
}

func TestSpinner(t *testing.T) {
	base := time.UnixMilli(0)
	// Each frame lasts spinnerFrameInterval (80ms); advancing by exactly
	// one interval must advance exactly one frame, and it must animate
	// (not return the same glyph forever).
	first := Spinner(base)
	second := Spinner(base.Add(80 * time.Millisecond))
	if first == second {
		t.Errorf("Spinner should change frame after one interval: got %q both times", first)
	}
	// A full cycle must return to the first frame.
	full := Spinner(base.Add(time.Duration(len(spinnerFrames)) * 80 * time.Millisecond))
	if full != first {
		t.Errorf("Spinner should wrap after a full cycle: got %q, want %q", full, first)
	}
}

func TestSpinnerNeverPanicsOnPre1970Time(t *testing.T) {
	Spinner(time.UnixMilli(-12345))
}
