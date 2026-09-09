package components

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGraphDimensions(t *testing.T) {
	series := []float64{1, 5, 2, 8, 3, 9, 1}
	for _, style := range []GraphStyle{GraphBraille, GraphBlock, GraphTTY} {
		lines := Graph(series, 10, 4, style, nil)
		if len(lines) != 4 {
			t.Errorf("%v: got %d lines, want 4", style, len(lines))
		}
		for _, l := range lines {
			if n := utf8.RuneCountInString(l); n != 10 {
				t.Errorf("%v: line width = %d, want 10 (line %q)", style, n, l)
			}
		}
	}
}

func TestGraphEmptySeries(t *testing.T) {
	lines := Graph(nil, 10, 4, GraphBlock, nil)
	if len(lines) != 4 {
		t.Fatalf("empty series: got %d lines, want 4 (all blank)", len(lines))
	}
}

func TestGraphBlockFillsBottomBeforeTop(t *testing.T) {
	// A steep ramp: leftmost column low, rightmost column at max.
	series := []float64{0, 100}
	lines := Graph(series, 2, 4, GraphBlock, nil)
	top, bottom := lines[0], lines[3]
	// Rightmost column (max value) must be fully filled at both extremes.
	if r := []rune(top)[1]; r != '█' {
		t.Errorf("top row, max column = %q, want full block", string(r))
	}
	if r := []rune(bottom)[1]; r != '█' {
		t.Errorf("bottom row, max column = %q, want full block", string(r))
	}
	// Leftmost column (value 0) must be empty at the top.
	if r := []rune(top)[0]; r != ' ' {
		t.Errorf("top row, zero column = %q, want space", string(r))
	}
}

func TestGraphTTYUsesASCIIOnly(t *testing.T) {
	series := []float64{0, 25, 50, 75, 100}
	lines := Graph(series, 12, 3, GraphTTY, nil)
	for _, l := range lines {
		for _, r := range l {
			if !strings.ContainsRune(" .:#", r) {
				t.Fatalf("tty mode produced a non-ASCII-safe rune %q in %q", r, l)
			}
		}
	}
}

func TestGraphBrailleFullColumnSetsAllDots(t *testing.T) {
	// 3 points -> 4 subcolumns (2 cells) at height 1 (4 subrows): the last
	// two subcolumns both land on the series max, filling the second cell's
	// 8 dots entirely; the series isn't flat overall (span > 0), so this
	// doesn't hit the flat-series baseline case.
	series := []float64{0, 100, 100}
	lines := Graph(series, 2, 1, GraphBraille, nil)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	r := []rune(lines[0])[1]
	// 0x28FF is the fully-filled 8-dot braille cell.
	if r != rune(0x28FF) {
		t.Errorf("second cell = %U, want U+28FF (fully filled)", r)
	}
}

func TestGraphStyleCycleAndParse(t *testing.T) {
	s := GraphBraille
	if s.Next() != GraphBlock || s.Next().Next() != GraphTTY || s.Next().Next().Next() != GraphBraille {
		t.Error("GraphStyle.Next() should cycle braille -> block -> tty -> braille")
	}
	for _, name := range []string{"braille", "block", "tty"} {
		got, ok := ParseGraphStyle(name)
		if !ok || got.String() != name {
			t.Errorf("ParseGraphStyle(%q) = (%v, %v)", name, got, ok)
		}
	}
	if _, ok := ParseGraphStyle("nonsense"); ok {
		t.Error("ParseGraphStyle should reject unknown names")
	}
}
