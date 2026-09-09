package components

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

func TestTreemapDimensions(t *testing.T) {
	items := []TreemapItem{{Size: 10, Color: "1"}, {Size: 5, Color: "2"}, {Size: 3, Color: "3"}}
	lines := Treemap(items, 20, 8, 0)
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want 8", len(lines))
	}
	for _, l := range lines {
		if n := lipgloss.Width(l); n != 20 {
			t.Errorf("line width = %d, want 20 (line %q)", n, l)
		}
	}
}

func TestTreemapEmptyOrZeroSize(t *testing.T) {
	if lines := Treemap(nil, 10, 4, 0); len(lines) != 4 {
		t.Fatalf("nil items: got %d lines, want 4", len(lines))
	}
	zero := []TreemapItem{{Size: 0, Color: "1"}, {Size: -1, Color: "2"}}
	if lines := Treemap(zero, 10, 4, 0); len(lines) != 4 {
		t.Fatalf("all-non-positive items: got %d lines, want 4", len(lines))
	}
}

func TestTreemapTwoEqualItemsSplitEvenly(t *testing.T) {
	items := []TreemapItem{{Size: 1, Color: "1"}, {Size: 1, Color: "2"}}
	lines := Treemap(items, 4, 2, 0)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	// Wider-than-tall container -> squarify splits into a left/right pair.
	for _, l := range lines {
		if lipgloss.Width(l) != 4 {
			t.Fatalf("line %q not 4 cells wide", l)
		}
	}
}

func TestTreemapRespectsMaxItems(t *testing.T) {
	items := make([]TreemapItem, 0, 100)
	for i := range 100 {
		items = append(items, TreemapItem{Size: float64(i + 1), Color: "1"})
	}
	// Should not panic and must still fill the requested dimensions even
	// when far fewer items are kept than were passed in.
	lines := Treemap(items, 10, 5, 5)
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
}

func TestSquarifyConservesArea(t *testing.T) {
	sizes := []float64{40, 30, 20, 10}
	rects := squarify(sizes, 0, 0, 10, 10)
	if len(rects) != len(sizes) {
		t.Fatalf("got %d rects, want %d", len(rects), len(sizes))
	}
	var total float64
	for _, r := range rects {
		total += r.w * r.h
	}
	if diff := total - 100; diff > 0.01 || diff < -0.01 {
		t.Errorf("total area = %v, want 100 (10x10)", total)
	}
}

func TestTopBySizeSortsDescendingAndCaps(t *testing.T) {
	items := []TreemapItem{{Size: 3}, {Size: 1}, {Size: 5}, {Size: 0}, {Size: -2}}
	got := topBySize(items, 2)
	if len(got) != 2 || got[0].Size != 5 || got[1].Size != 3 {
		t.Fatalf("topBySize = %+v, want [5, 3]", got)
	}
}

// TestIsBorderCell is the core of the "same-colour neighbours are
// indistinguishable" fix: every cell on a rectangle's edge — including the
// outer edge of the whole treemap — must be flagged as a border cell so it
// gets darkened, distinguishing it from same-coloured neighbours the way
// mempool.space's own grid lines do.
func TestIsBorderCell(t *testing.T) {
	// 3x3 grid, one rectangle filling it entirely.
	grid := [][]int{{0, 0, 0}, {0, 0, 0}, {0, 0, 0}}
	cases := []struct {
		x, y int
		want bool
	}{
		{0, 0, true}, {1, 0, true}, {2, 0, true},
		{0, 1, true}, {1, 1, false}, {2, 1, true}, // only the center is interior
		{0, 2, true}, {1, 2, true}, {2, 2, true},
	}
	for _, c := range cases {
		if got := isBorderCell(grid, c.x, c.y, 3, 3, 0); got != c.want {
			t.Errorf("isBorderCell(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestIsBorderCellBetweenTwoRects(t *testing.T) {
	// 4x1 grid: left half is rect 0, right half is rect 1 — the seam
	// between them must be flagged on both sides even though they're
	// adjacent, same-height rectangles (the exact case a flat fill would
	// render as one indistinguishable blob if same-coloured).
	grid := [][]int{{0, 0, 1, 1}}
	if !isBorderCell(grid, 1, 0, 4, 1, 0) {
		t.Error("cell on rect 0's side of the seam should be a border cell")
	}
	if !isBorderCell(grid, 2, 0, 4, 1, 1) {
		t.Error("cell on rect 1's side of the seam should be a border cell")
	}
}

func TestDarkenColorReducesLightness(t *testing.T) {
	orig := lipgloss.Color("#A3BE8C")
	dark := darkenColor(orig, 0.5)
	if dark == orig {
		t.Fatal("darkenColor should produce a different colour")
	}
	origCol, err := colorful.Hex(string(orig))
	if err != nil {
		t.Fatal(err)
	}
	darkCol, err := colorful.Hex(string(dark))
	if err != nil {
		t.Fatal(err)
	}
	_, _, origL := origCol.Hsl()
	_, _, darkL := darkCol.Hsl()
	if darkL >= origL {
		t.Errorf("darkened lightness %v should be less than original %v", darkL, origL)
	}
}

func TestDarkenColorFallsBackOnUnparseable(t *testing.T) {
	if got := darkenColor("not-a-hex-colour", 0.5); got != "not-a-hex-colour" {
		t.Errorf("expected passthrough for an unparseable colour, got %v", got)
	}
}
