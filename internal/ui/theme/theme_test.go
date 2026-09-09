package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func eqHex(a, b lipgloss.Color) bool {
	return strings.EqualFold(string(a), string(b))
}

func TestGradientEndpoints(t *testing.T) {
	if got := Nord.Gradient(0); !eqHex(got, Nord.Good) {
		t.Errorf("Gradient(0) = %v, want Good (%v)", got, Nord.Good)
	}
	if got := Nord.Gradient(1); !eqHex(got, Nord.Bad) {
		t.Errorf("Gradient(1) = %v, want Bad (%v)", got, Nord.Bad)
	}
}

func TestGradientClampsOutOfRange(t *testing.T) {
	if !eqHex(Nord.Gradient(-5), Nord.Good) {
		t.Error("Gradient should clamp negative fractions to Good")
	}
	if !eqHex(Nord.Gradient(5), Nord.Bad) {
		t.Error("Gradient should clamp fractions above 1 to Bad")
	}
}

func TestGradientNoColourTheme(t *testing.T) {
	nc := Theme{Name: "no-colour"}
	if got := nc.Gradient(0.5); got != "" {
		t.Errorf("no-colour theme Gradient() = %q, want empty", got)
	}
}

func TestGradientIsMonotonicInLuminanceDirection(t *testing.T) {
	// Not asserting exact colours mid-ramp (that's the library's job) — just
	// that distinct fractions produce distinct colours, i.e. it's really
	// blending rather than snapping to one of the three stops everywhere.
	a := Nord.Gradient(0.1)
	b := Nord.Gradient(0.4)
	c := Nord.Gradient(0.9)
	if a == b || b == c || a == c {
		t.Errorf("expected distinct colours across the ramp, got %v %v %v", a, b, c)
	}
}
