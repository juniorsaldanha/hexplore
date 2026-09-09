package components

import (
	"math"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// TreemapItem is one rectangle's worth of data — mempool.space calls this
// the "goggles" block visualization: transactions packed by size, coloured
// by fee rate (or, for us, by classification).
type TreemapItem struct {
	Size  float64
	Color lipgloss.Color
}

// Treemap lays out items into a width x height grid using the squarified
// algorithm (Bruls, Huizing & van Wijk, 2000) — the same one mempool.space's
// own visualization uses — and rasterizes it into height rows of width
// full-block cells. Items with non-positive size are dropped; only the
// largest maxItems (by size) are laid out, since a busy mempool has far
// more transactions than a terminal has cells for.
func Treemap(items []TreemapItem, width, height, maxItems int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	items = topBySize(items, maxItems)
	if len(items) == 0 {
		return make([]string, height)
	}

	var total float64
	for _, it := range items {
		total += it.Size
	}
	if total <= 0 {
		return make([]string, height)
	}

	area := float64(width) * float64(height)
	scaled := make([]float64, len(items))
	for i, it := range items {
		scaled[i] = it.Size / total * area
	}

	rects := squarify(scaled, 0, 0, float64(width), float64(height))
	return rasterize(items, rects, width, height)
}

// topBySize keeps only positive-size items, sorted descending, capped to n
// (n<=0 means unlimited).
func topBySize(items []TreemapItem, n int) []TreemapItem {
	out := make([]TreemapItem, 0, len(items))
	for _, it := range items {
		if it.Size > 0 {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

type treemapRect struct{ x, y, w, h float64 }

// squarify recursively fills (x, y, w, h) with rectangles proportional to
// sizes, which must already sum to w*h. Each "row" is grown one item at a
// time along the container's shorter side for as long as doing so improves
// (lowers) the worst aspect ratio in the row, then laid out, shrinking the
// remaining rectangle for the next row.
func squarify(sizes []float64, x, y, w, h float64) []treemapRect {
	rects := make([]treemapRect, 0, len(sizes))
	i := 0
	for i < len(sizes) {
		shortSide := min(w, h)

		rowEnd := i + 1
		rowSum := sizes[i]
		bestWorst := worstRatio(rowSum, shortSide, rowSum)
		for rowEnd < len(sizes) {
			candidateSum := rowSum + sizes[rowEnd]
			candidateWorst := worstRatioRow(sizes[i:rowEnd+1], shortSide, candidateSum)
			if candidateWorst > bestWorst {
				break
			}
			rowSum, bestWorst = candidateSum, candidateWorst
			rowEnd++
		}
		row := sizes[i:rowEnd]

		if w >= h {
			rowWidth := rowSum / h
			ry := y
			for _, s := range row {
				rh := s / rowWidth
				rects = append(rects, treemapRect{x, ry, rowWidth, rh})
				ry += rh
			}
			x += rowWidth
			w -= rowWidth
		} else {
			rowHeight := rowSum / w
			rx := x
			for _, s := range row {
				rw := s / rowHeight
				rects = append(rects, treemapRect{rx, y, rw, rowHeight})
				rx += rw
			}
			y += rowHeight
			h -= rowHeight
		}
		i = rowEnd
	}
	return rects
}

// worstRatioRow is worstRatio generalized to a multi-item row.
func worstRatioRow(row []float64, side, sum float64) float64 {
	if len(row) == 1 {
		return worstRatio(row[0], side, sum)
	}
	if sum <= 0 || side <= 0 {
		return math.Inf(1)
	}
	thickness := sum / side
	worst := 0.0
	for _, s := range row {
		length := s / thickness
		worst = max(worst, max(length/thickness, thickness/length))
	}
	return worst
}

func worstRatio(size, side, sum float64) float64 {
	if sum <= 0 || side <= 0 {
		return math.Inf(1)
	}
	thickness := sum / side
	length := size / thickness
	return max(length/thickness, thickness/length)
}

func rasterize(items []TreemapItem, rects []treemapRect, width, height int) []string {
	grid := make([][]int, height)
	for y := range grid {
		grid[y] = make([]int, width)
		for x := range grid[y] {
			grid[y][x] = -1
		}
	}
	for i, r := range rects {
		x0, y0 := int(math.Round(r.x)), int(math.Round(r.y))
		x1, y1 := int(math.Round(r.x+r.w)), int(math.Round(r.y+r.h))
		for y := max(y0, 0); y < min(y1, height); y++ {
			for x := max(x0, 0); x < min(x1, width); x++ {
				grid[y][x] = i
			}
		}
	}

	// A rectangle rendered as flat, solid colour is indistinguishable from
	// its neighbour whenever they land on the same fee-tier colour (a very
	// common case — most of a block is "normal" fee, i.e. green) — exactly
	// what mempool.space's own grid-line texture between squares avoids.
	// Darkening each rectangle's outer ring of cells reproduces that
	// border without needing per-cell background colours.
	const borderDarken = 0.5
	darkened := make([]lipgloss.Color, len(items))
	for i, it := range items {
		darkened[i] = darkenColor(it.Color, borderDarken)
	}

	lines := make([]string, height)
	for y := range height {
		var b strings.Builder
		for x := range width {
			idx := grid[y][x]
			if idx < 0 {
				b.WriteByte(' ')
				continue
			}
			color := items[idx].Color
			if isBorderCell(grid, x, y, width, height, idx) {
				color = darkened[idx]
			}
			b.WriteString(lipgloss.NewStyle().Foreground(color).Render("█"))
		}
		lines[y] = b.String()
	}
	return lines
}

// isBorderCell reports whether (x, y) sits on the edge of its rectangle —
// any orthogonal neighbour belonging to a different rectangle (or off the
// grid entirely) — versus its interior.
func isBorderCell(grid [][]int, x, y, width, height, idx int) bool {
	if x == 0 || x == width-1 || y == 0 || y == height-1 {
		return true // outer edge of the whole treemap gets a border too
	}
	return grid[y-1][x] != idx || grid[y+1][x] != idx || grid[y][x-1] != idx || grid[y][x+1] != idx
}

// darkenColor reduces a hex colour's lightness by frac (0-1) in HSL space.
// Falls back to the original colour if it isn't parseable hex (shouldn't
// happen for anything coming out of package theme).
func darkenColor(c lipgloss.Color, frac float64) lipgloss.Color {
	col, err := colorful.Hex(string(c))
	if err != nil {
		return c
	}
	h, s, l := col.Hsl()
	col = colorful.Hsl(h, s, l*(1-frac))
	return lipgloss.Color(col.Clamped().Hex())
}
