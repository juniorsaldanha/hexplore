package views

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

// Loading is the full-screen placeholder shown while a screen's initial
// fetch is in flight — an animated spinner rather than a blank screen or,
// worse, the real view rendered on zero-valued data (docs/PLAN.md §11:
// "never block on the network... dim spinners on pending panels").
func Loading(label string, th theme.Theme) string {
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	return "\n  " + accent.Render(components.Spinner(time.Now())) + dim.Render("  loading "+label+"…")
}

// loadingLine is Loading's single-line form, for a panel that's still
// progressively loading while the rest of its screen is already showing
// (the block treemap, an address's tx list/balance history) — an accented
// spinner glyph rather than plain dim text, so it's visibly alive.
func loadingLine(label string, th theme.Theme) string {
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	return accent.Render(components.Spinner(time.Now())) + dim.Render("  loading "+label+"…")
}
