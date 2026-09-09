// Package theme holds the colour palette. One green/amber/red ramp drives
// both fee tiers and mempool congestion so unrelated panels read as one
// system (docs/PLAN.md §11). Honours NO_COLOR.
package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

type Theme struct {
	Name string

	Fg, Dim, Border lipgloss.Color
	Accent          lipgloss.Color // panel titles, focus ring
	Good, Warn, Bad lipgloss.Color // fee tiers / congestion ramp
	TestnetAccent   lipgloss.Color
}

var Nord = Theme{
	Name:          "nord",
	Fg:            "#D8DEE9",
	Dim:           "#4C566A",
	Border:        "#3B4252",
	Accent:        "#88C0D0",
	Good:          "#A3BE8C",
	Warn:          "#EBCB8B",
	Bad:           "#BF616A",
	TestnetAccent: "#D08770",
}

var Dracula = Theme{
	Name:          "dracula",
	Fg:            "#F8F8F2",
	Dim:           "#6272A4",
	Border:        "#44475A",
	Accent:        "#BD93F9",
	Good:          "#50FA7B",
	Warn:          "#F1FA8C",
	Bad:           "#FF5555",
	TestnetAccent: "#FFB86C",
}

var Solarized = Theme{
	Name:          "solarized",
	Fg:            "#839496",
	Dim:           "#586E75",
	Border:        "#073642",
	Accent:        "#268BD2",
	Good:          "#859900",
	Warn:          "#B58900",
	Bad:           "#DC322F",
	TestnetAccent: "#CB4B16",
}

// noColor strips every colour so pipes and NO_COLOR terminals get plain text.
var noColor = Theme{
	Name: "no-colour",
}

// All is the cycle order for the `t` keybinding.
var All = []Theme{Nord, Dracula, Solarized}

// Active returns the first built-in theme, or an uncoloured theme when
// NO_COLOR is set.
func Active() Theme {
	if os.Getenv("NO_COLOR") != "" {
		return noColor
	}
	return Nord
}

// Next cycles to the theme after current in All, wrapping around. Falls
// back to All[0] if current isn't one of them (e.g. the NO_COLOR theme).
func Next(current Theme) Theme {
	for i, t := range All {
		if t.Name == current.Name {
			return All[(i+1)%len(All)]
		}
	}
	return All[0]
}

// FeeColor maps a sat/vB rate onto the tier ramp — the same thresholds used
// for the mempool congestion bar so both read consistently.
func (t Theme) FeeColor(satVB float64) lipgloss.Color {
	switch {
	case satVB >= 50:
		return t.Bad
	case satVB >= 15:
		return t.Warn
	default:
		return t.Good
	}
}

// Gradient blends Good -> Warn -> Bad in perceptual (HCL) space at frac
// (0 = Good, 0.5 = Warn, 1 = Bad) — the btop-style low-to-high colour ramp
// graphs and gauges are drawn with. Returns "" (no colour) for the
// no-colour/NO_COLOR theme, since it has no ramp colours to blend.
func (t Theme) Gradient(frac float64) lipgloss.Color {
	if t.Good == "" {
		return ""
	}
	frac = min(max(frac, 0), 1)
	good, gErr := colorful.Hex(string(t.Good))
	warn, wErr := colorful.Hex(string(t.Warn))
	bad, bErr := colorful.Hex(string(t.Bad))
	if gErr != nil || wErr != nil || bErr != nil {
		return t.FeeColor(frac * 100) // shouldn't happen with the built-in themes; degrade to discrete tiers
	}
	var c colorful.Color
	if frac < 0.5 {
		c = good.BlendHcl(warn, frac/0.5)
	} else {
		c = warn.BlendHcl(bad, (frac-0.5)/0.5)
	}
	return lipgloss.Color(c.Hex())
}
