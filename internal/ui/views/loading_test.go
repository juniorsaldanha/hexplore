package views

import (
	"strings"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func TestLoadingContainsLabelAndASpinnerGlyph(t *testing.T) {
	out := Loading("block", theme.Nord)
	if !strings.Contains(out, "loading block") {
		t.Errorf("Loading output missing label: %q", out)
	}
	glyphs := "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	if !strings.ContainsAny(out, glyphs) {
		t.Errorf("Loading output missing a spinner glyph: %q", out)
	}
}
