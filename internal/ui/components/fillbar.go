// Package components holds small, stateless render functions shared across
// views: fill bars, sparklines, and their supporting formatting helpers.
package components

import "strings"

// eighths are the eighth-block runes, one-eighth of a cell each — 160 steps
// of precision on a 20-cell bar instead of 20 (docs/PLAN.md §11).
var eighths = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// FillBar renders pct (0-1) across width cells at eighth-cell resolution.
func FillBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	if width <= 0 {
		return ""
	}
	totalEighths := int(pct*float64(width)*8 + 0.5)
	full := totalEighths / 8
	rem := totalEighths % 8
	if full > width {
		full, rem = width, 0
	}

	var b strings.Builder
	b.WriteString(strings.Repeat(string(eighths[8]), full))
	if full < width && rem > 0 {
		b.WriteRune(eighths[rem])
		full++
	}
	b.WriteString(strings.Repeat(" ", width-full))
	return b.String()
}
