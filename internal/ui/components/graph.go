package components

import "github.com/charmbracelet/lipgloss"

// GraphStyle picks how Graph renders a series — btop's three symbol sets,
// from highest to lowest resolution / broadest terminal support.
type GraphStyle int

const (
	GraphBraille GraphStyle = iota // 2x4 dots per cell — highest resolution, needs a Unicode-braille-capable font
	GraphBlock                     // eighth-block runes — the same resolution trick as FillBar
	GraphTTY                       // plain ASCII (' ', '.', ':', '#') — works on any terminal
)

func (s GraphStyle) Next() GraphStyle { return (s + 1) % 3 }

func (s GraphStyle) String() string {
	switch s {
	case GraphBraille:
		return "braille"
	case GraphTTY:
		return "tty"
	default:
		return "block"
	}
}

func ParseGraphStyle(s string) (GraphStyle, bool) {
	switch s {
	case "braille":
		return GraphBraille, true
	case "block":
		return GraphBlock, true
	case "tty":
		return GraphTTY, true
	}
	return GraphBlock, false
}

// Graph renders series as a filled area chart, width columns by height rows,
// scaled to series' own min/max. colorAt maps a row's fractional height
// (0 = bottom, 1 = top) to a color, so callers can apply a low-to-high
// gradient (green-to-red) the way btop colors its graphs.
func Graph(series []float64, width, height int, style GraphStyle, colorAt func(rowFrac float64) lipgloss.Color) []string {
	if width <= 0 || height <= 0 || len(series) == 0 {
		return make([]string, max(height, 0))
	}

	switch style {
	case GraphBraille:
		return graphBraille(series, width, height, colorAt)
	case GraphTTY:
		return graphLeveled(series, width, height, colorAt, ttyRow)
	default:
		return graphLeveled(series, width, height, colorAt, blockRow)
	}
}

// levels returns, per column, how many eighths of the full height*8 scale
// are filled — the shared vertical-scaling step block and tty modes rely on.
func levels(series []float64, width, height int) []int {
	buckets := resample(series, width)
	min, max := buckets[0], buckets[0]
	for _, v := range buckets {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	total := height * 8
	out := make([]int, width)
	for i, v := range buckets {
		if span == 0 {
			out[i] = total / 8 // flat series: draw a low, level line rather than nothing
			continue
		}
		out[i] = int((v-min)/span*float64(total) + 0.5)
	}
	return out
}

type rowRenderer func(filledEighths int) rune

func blockRow(filled int) rune {
	switch {
	case filled <= 0:
		return ' '
	case filled >= 8:
		return '█'
	default:
		return eighths[filled]
	}
}

func ttyRow(filled int) rune {
	switch {
	case filled <= 0:
		return ' '
	case filled >= 8:
		return '#'
	case filled >= 4:
		return ':'
	default:
		return '.'
	}
}

func graphLeveled(series []float64, width, height int, colorAt func(float64) lipgloss.Color, render rowRenderer) []string {
	fill := levels(series, width, height)
	lines := make([]string, height)
	for r := range height {
		rowFromBottom := height - 1 - r
		style := lipgloss.NewStyle()
		if colorAt != nil {
			frac := 0.0
			if height > 1 {
				frac = float64(rowFromBottom) / float64(height-1)
			}
			style = style.Foreground(colorAt(frac))
		}
		runes := make([]rune, width)
		for c := range width {
			cell := clamp(fill[c]-rowFromBottom*8, 0, 8)
			runes[c] = render(cell)
		}
		lines[r] = style.Render(string(runes))
	}
	return lines
}

// Braille dot bits, top-to-bottom within a cell, standard Unicode braille
// numbering (matches the drawille/braille-graphing convention).
var brailleLeftBits = [4]byte{0x01, 0x02, 0x04, 0x40}
var brailleRightBits = [4]byte{0x08, 0x10, 0x20, 0x80}

func graphBraille(series []float64, width, height int, colorAt func(float64) lipgloss.Color) []string {
	subFill := levels(series, width*2, height) // 2 subcolumns per cell, still height*8... need height*4 scale below
	// levels() scales to height*8; braille needs height*4 subrows, so halve.
	for i := range subFill {
		subFill[i] /= 2
	}

	lines := make([]string, height)
	for r := range height {
		rowFromBottom := height - 1 - r
		style := lipgloss.NewStyle()
		if colorAt != nil {
			frac := 0.0
			if height > 1 {
				frac = float64(rowFromBottom) / float64(height-1)
			}
			style = style.Foreground(colorAt(frac))
		}
		runes := make([]rune, width)
		for c := range width {
			var b byte
			for sub := range 2 {
				filledInBand := clamp(subFill[c*2+sub]-rowFromBottom*4, 0, 4)
				bits := brailleLeftBits
				if sub == 1 {
					bits = brailleRightBits
				}
				for row := 4 - filledInBand; row < 4; row++ {
					b |= bits[row]
				}
			}
			runes[c] = rune(0x2800 + int(b))
		}
		lines[r] = style.Render(string(runes))
	}
	return lines
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
