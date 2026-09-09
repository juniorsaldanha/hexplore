// Package views renders read-only screens from plain data structs — no
// provider types ever reach here, only domain and primitives.
package views

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

// Unit is the `u`-key display toggle (docs/PLAN.md §8). Only the dashboard
// honours it today — block/tx/address views still show sats/BTC directly.
type Unit int

const (
	UnitBTC Unit = iota
	UnitSat
	UnitFiat
)

func (u Unit) Next() Unit { return (u + 1) % 3 }

func (u Unit) String() string {
	switch u {
	case UnitSat:
		return "sats"
	case UnitFiat:
		return "fiat"
	default:
		return "btc"
	}
}

// formatAmount renders sats in the active unit. btcDecimals lets each panel
// keep its own BTC precision (a fee total wants more places than a
// dashboard headline); sat and fiat modes always use a fixed precision.
func formatAmount(sats int64, unit Unit, btcDecimals int, price float64, currency string) string {
	switch unit {
	case UnitSat:
		return formatInt(int(sats)) + " sats"
	case UnitFiat:
		if price <= 0 {
			return "—"
		}
		return currency + " " + formatFloat(float64(sats)/1e8*price, 2)
	default:
		return formatFloat(float64(sats)/1e8, btcDecimals) + " BTC"
	}
}

type DashboardData struct {
	Units      Unit
	GraphStyle components.GraphStyle

	ProviderName string
	Degraded     bool // active host isn't the most-preferred one in the fallback chain
	Network      string
	TipHeight    int
	LastBlockAt  time.Time

	HasPrice        bool
	PriceConfigured bool // a PriceProvider exists but hasn't returned data yet — distinct from none being configured at all
	Price           domain.Price
	PriceHistory    []float64

	Fees       domain.FeeTiers
	Mempool    domain.MempoolState
	NextBlocks []domain.ProjectedBlock
	Blocks     []domain.Block
	Cursor     int

	Currency     string
	AssumedVSize int
	Caps         domain.Capabilities

	Stale map[string]time.Duration // panel -> how long since last good fetch; absent = fresh

	FPS int // render-loop frames in the last second — 0 before the first tick lands

	Width, Height int
}

// box is a panel's content before it's wrapped in a border — kept as
// separate lines so a row of panels can be padded to equal height before
// rendering, keeping their bottom borders aligned.
type box struct {
	title string
	lines []string
}

func (b box) height() int { return len(b.lines) }

func padRow(boxes ...*box) {
	maxLines := 0
	for _, b := range boxes {
		maxLines = max(maxLines, b.height())
	}
	for _, b := range boxes {
		for len(b.lines) < maxLines {
			b.lines = append(b.lines, "")
		}
	}
}

func renderBox(b box, width int, border, title lipgloss.Style) string {
	content := title.Render(b.title) + "\n" + strings.Join(b.lines, "\n")
	return border.Width(width).Render(content)
}

// innerWidth is the text budget inside a bordered, 1-cell-padded box of the
// given outer width — 2 for the border, 2 for the padding.
func innerWidth(width int) int {
	return max(width-4, 4)
}

func Dashboard(d DashboardData, th theme.Theme) string {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(0, 1)
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)

	stacked := d.Width < 100
	compact := d.Width < 80 // drop sparkline, pool column, and the block list's second line

	panelWidth := 28
	if !stacked {
		panelWidth = (d.Width - 8) / 3
	} else if d.Width > 0 {
		panelWidth = d.Width - 4
	}

	priceB := priceBox(d, th, dim, innerWidth(panelWidth), compact)
	feeB := feeMarketBox(d, th, dim)
	mempoolB := mempoolBox(d, th, dim, innerWidth(panelWidth))
	if !stacked {
		padRow(&priceB, &feeB, &mempoolB)
	}

	var topRow string
	if stacked {
		topRow = lipgloss.JoinVertical(lipgloss.Left,
			renderBox(priceB, panelWidth, border, title),
			renderBox(feeB, panelWidth, border, title),
			renderBox(mempoolB, panelWidth, border, title))
	} else {
		topRow = lipgloss.JoinHorizontal(lipgloss.Top,
			renderBox(priceB, panelWidth, border, title),
			renderBox(feeB, panelWidth, border, title),
			renderBox(mempoolB, panelWidth, border, title))
	}

	nextWidth := panelWidth
	listWidth := panelWidth
	if !stacked {
		nextWidth = (d.Width - 6) * 3 / 10
		listWidth = d.Width - 6 - nextWidth
	}
	nextB := nextBlockBox(d, th, innerWidth(nextWidth))
	listB := latestBlocksBox(d, th, dim, innerWidth(listWidth), compact)
	if !stacked {
		padRow(&nextB, &listB)
	}

	var bottomRow string
	if stacked {
		bottomRow = lipgloss.JoinVertical(lipgloss.Left,
			renderBox(nextB, nextWidth, border, title),
			renderBox(listB, listWidth, border, title))
	} else {
		bottomRow = lipgloss.JoinHorizontal(lipgloss.Top,
			renderBox(nextB, nextWidth, border, title),
			renderBox(listB, listWidth, border, title))
	}

	header := dashboardHeader(d, th)
	footer := footerLine(d, th, dim)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", topRow, "", bottomRow, "", footer)
}

func dashboardHeader(d DashboardData, th theme.Theme) string {
	networkStyle := lipgloss.NewStyle().Foreground(th.Fg)
	if d.Network != "" && d.Network != "mainnet" {
		networkStyle = lipgloss.NewStyle().Foreground(th.TestnetAccent).Bold(true)
	}
	ago := "—"
	if !d.LastBlockAt.IsZero() {
		ago = formatAge(time.Since(d.LastBlockAt))
	}
	provider := d.ProviderName
	if d.Degraded {
		provider += lipgloss.NewStyle().Foreground(th.Warn).Render(" · degraded")
	}
	return fmt.Sprintf(" hexplore   %s · %s · ● %s ago · %s",
		provider, networkStyle.Render(d.Network), ago, formatInt(d.TipHeight))
}

// footerLine right-aligns a live fps readout against the keybinding hints —
// green/amber/red by how close the render loop is keeping to targetFPS(60).
func footerLine(d DashboardData, th theme.Theme, dim lipgloss.Style) string {
	hints := " / search   : command   ↵ open   r refresh   c config   ? help   q quit"
	if d.Width <= 0 {
		return dim.Render(hints)
	}

	fpsFrac := float64(d.FPS) / 60
	fpsColor := lipgloss.NewStyle().Foreground(th.Gradient(1 - fpsFrac)) // 60fps -> frac 0 -> Good; 0fps -> frac 1 -> Bad
	fpsText := fmt.Sprintf("%d fps", d.FPS)

	gap := d.Width - lipgloss.Width(hints) - lipgloss.Width(fpsText) - 1
	if gap < 1 {
		return dim.Render(hints)
	}
	return dim.Render(hints) + strings.Repeat(" ", gap) + fpsColor.Render(fpsText)
}

// changeArrow picks the up/down/flat arrow and colour for a 24h change —
// green/red/grey, grey only for an exact zero rather than lumping it in
// with "down".
func changeArrow(change24h float64, th theme.Theme) (arrow string, style lipgloss.Style) {
	switch {
	case change24h > 0:
		return "▲", lipgloss.NewStyle().Foreground(th.Good)
	case change24h < 0:
		return "▼", lipgloss.NewStyle().Foreground(th.Bad)
	default:
		return "→", lipgloss.NewStyle().Foreground(th.Dim)
	}
}

func priceBox(d DashboardData, th theme.Theme, dim lipgloss.Style, width int, compact bool) box {
	b := box{title: fmt.Sprintf("BTC/%s", d.Currency) + staleSuffix(d, "price", dim)}
	if !d.HasPrice {
		if d.PriceConfigured {
			b.lines = []string{loadingLine("price", th)}
		} else {
			b.lines = []string{"price unavailable", dim.Render("no PriceProvider configured")}
		}
		return b
	}
	arrow, changeStyle := changeArrow(d.Price.Change24h, th)
	changeLine := changeStyle.Render(fmt.Sprintf("%s %.2f%%", arrow, d.Price.Change24h))

	whole := formatInt(int(d.Price.Value))
	centsStr := fmt.Sprintf(".%02d", int(math.Round((d.Price.Value-math.Floor(d.Price.Value))*100))%100)
	// compact terminals already shed richness (sparkline, pool column) to
	// stay short — the 5-row big-font tile adds real height to the whole
	// top row via padRow, so it rides the same breakpoint rather than
	// growing an already-tight layout further.
	if !compact && components.BigTextWidth(whole)+len(centsStr) <= width {
		rows := components.BigText(whole)
		bigStyle := lipgloss.NewStyle().Bold(true).Foreground(changeStyle.GetForeground())
		b.lines = append(b.lines, changeLine, "")
		for i, row := range rows {
			line := bigStyle.Render(row)
			if i == len(rows)-1 {
				line += dim.Render(centsStr)
			}
			b.lines = append(b.lines, line)
		}
	} else {
		b.lines = []string{fmt.Sprintf("$%s   %s", formatFloat(d.Price.Value, 2), changeLine)}
	}
	if !compact && len(d.PriceHistory) > 1 {
		const graphHeight = 3
		b.lines = append(b.lines, components.Graph(d.PriceHistory, width, graphHeight, d.GraphStyle, th.Gradient)...)
	}
	if len(d.PriceHistory) > 0 {
		lo, hi := minMax(d.PriceHistory)
		b.lines = append(b.lines, dim.Render(fmt.Sprintf("24h  L %s  H %s", formatFloat(lo, 0), formatFloat(hi, 0))))
	}
	return b
}

func feeMarketBox(d DashboardData, th theme.Theme, dim lipgloss.Style) box {
	row := func(label string, satVB float64) string {
		fiat := satVB * float64(d.AssumedVSize) * d.Price.Value / 100_000_000
		fiatStr := "—"
		if d.HasPrice {
			fiatStr = d.Currency + " " + formatFloat(fiat, 2)
		}
		c := lipgloss.NewStyle().Foreground(th.FeeColor(satVB))
		return fmt.Sprintf("%-9s %s  %s", label, c.Render(formatFloat(satVB, 0)+" sat/vB"), fiatStr)
	}
	lines := []string{
		row("High", d.Fees.HighSatVB),
		row("Avg", d.Fees.AvgSatVB),
		row("Low", d.Fees.LowSatVB),
	}
	if d.Fees.EconomySatVB > 0 {
		lines = append(lines, row("Economy", d.Fees.EconomySatVB))
	}
	// Two short lines rather than one long one: at the panel widths this
	// box actually renders at, "per N vB tx  ·  recommended by <provider>"
	// is long enough to wrap inside the bordered box — an extra line
	// padRow never counts, which staircases this panel's border against
	// its neighbours.
	lines = append(lines,
		dim.Render(fmt.Sprintf("per %d vB tx", d.AssumedVSize)),
		dim.Render("recommended by "+d.ProviderName))
	return box{
		title: "FEE MARKET" + staleSuffix(d, "fees", dim),
		lines: lines,
	}
}

func mempoolBox(d DashboardData, th theme.Theme, dim lipgloss.Style, width int) box {
	vmb := float64(d.Mempool.VSize) / 1_000_000
	blocksBacklog := vmb // 1 vMB ≈ 1 block
	congestionFrac := min(blocksBacklog/8, 1)
	congestion := "low"
	switch {
	case blocksBacklog > 6:
		congestion = "severe"
	case blocksBacklog > 3:
		congestion = "moderate"
	case blocksBacklog > 1:
		congestion = "mild"
	}
	suffix := fmt.Sprintf("  %.1f blk", blocksBacklog)
	barStyle := lipgloss.NewStyle().Foreground(th.Gradient(congestionFrac))
	bar := barStyle.Render(components.FillBar(congestionFrac, fitBar(width, suffix)))
	lines := []string{
		fmt.Sprintf("%s tx    %.2f vMB", formatInt(d.Mempool.Count), vmb),
		bar + suffix,
		fmt.Sprintf("total fee    %s", formatAmount(d.Mempool.TotalFeeSats, d.Units, 3, d.Price.Value, d.Currency)),
		"congestion: " + congestion,
	}
	if hist := feeHistogramSeries(d.Mempool.Histogram); len(hist) > 1 {
		const histHeight = 3
		lines = append(lines, dim.Render("fee distribution (low → high)"))
		lines = append(lines, components.Graph(hist, width, histHeight, d.GraphStyle, th.Gradient)...)
	}
	return box{title: "MEMPOOL", lines: lines}
}

// feeHistogramSeries reorders the mempool's [feerate, vsize] histogram
// (stored descending by feerate) into ascending order so the mini-graph
// reads low-fee-left to high-fee-right, like a typical distribution chart.
func feeHistogramSeries(histogram [][2]float64) []float64 {
	out := make([]float64, len(histogram))
	for i, bucket := range histogram {
		out[len(histogram)-1-i] = bucket[1]
	}
	return out
}

func nextBlockBox(d DashboardData, th theme.Theme, width int) box {
	prefix := ""
	if !d.Caps.NextBlockExact {
		prefix = "~"
	}
	b := box{title: fmt.Sprintf("NEXT BLOCK  %s%s", prefix, formatAge(estBlockETA(d)))}
	if len(d.NextBlocks) == 0 {
		b.lines = []string{loadingLine("mempool data", th)}
		return b
	}
	b0 := d.NextBlocks[0]
	fill := float64(b0.VSize) / 1_000_000
	pctSuffix := fmt.Sprintf("  %3.0f%%", fill*100)
	bar0 := lipgloss.NewStyle().Foreground(th.Gradient(fill)).Render(components.FillBar(fill, fitBar(width, pctSuffix)))
	b.lines = []string{
		fmt.Sprintf("%s%s sat/vB    %s – %s", prefix, formatFloat(b0.MedianFeeRate, 0), formatFloat(b0.MinFeeRate, 0), formatFloat(b0.MaxFeeRate, 0)),
		fmt.Sprintf("%s MB     %s%s tx", formatFloat(fill, 2), prefix, formatInt(b0.NTx)),
		fmt.Sprintf("%s%s fees", prefix, formatAmount(b0.TotalFeeSats, d.Units, 4, d.Price.Value, d.Currency)),
		bar0 + pctSuffix,
	}
	for i, nb := range d.NextBlocks[1:] {
		f := float64(nb.VSize) / 1_000_000
		lead := fmt.Sprintf("+%d  %s%s sat/vB  ", i+1, prefix, formatFloat(nb.MedianFeeRate, 0))
		tail := fmt.Sprintf(" %3.0f%%", f*100)
		bar := lipgloss.NewStyle().Foreground(th.Gradient(f)).Render(components.FillBar(f, fitBar(width, lead+tail)))
		b.lines = append(b.lines, lead+bar+tail)
	}
	return b
}

// fitBar returns how many cells are left for a fill bar once fixed-width
// text (already measured, ANSI-aware) shares the same line.
func fitBar(width int, fixedText string) int {
	return max(width-lipgloss.Width(fixedText), 3)
}

func estBlockETA(d DashboardData) time.Duration {
	if d.LastBlockAt.IsZero() {
		return 0
	}
	elapsed := time.Since(d.LastBlockAt)
	return max(10*time.Minute-elapsed, 0)
}

func latestBlocksBox(d DashboardData, th theme.Theme, dim lipgloss.Style, width int, compact bool) box {
	selected := lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	b := box{title: "LATEST BLOCKS"}
	if len(d.Blocks) == 0 {
		b.lines = []string{loadingLine("blocks", th)}
		return b
	}
	for i, blk := range d.Blocks {
		if i > 0 {
			b.lines = append(b.lines, "")
		}
		fill := float64(blk.Weight) / 4_000_000
		lead := fmt.Sprintf("%s   %s   ", formatInt(blk.Height), formatAge(time.Since(blk.Time)))
		tail := fmt.Sprintf(" %3.0f%%", fill*100)
		if !compact && blk.Pool != "" {
			tail += "  " + blk.Pool
		}
		rawBar := components.FillBar(fill, fitBar(width, lead+tail))
		var line1 string
		if i == d.Cursor {
			// Nesting the gradient-coloured bar inside the selected style
			// would reset styling partway through the line (ANSI codes
			// don't nest cleanly) — keep the selected row a single solid
			// colour instead.
			line1 = selected.Render(lead + rawBar + tail)
		} else {
			bar := lipgloss.NewStyle().Foreground(th.Gradient(fill)).Render(rawBar)
			line1 = lead + bar + tail
		}
		b.lines = append(b.lines, line1)
		if !compact {
			feeStr, avgRate := "fee —", "—"
			if blk.Fee != nil {
				p := ""
				if blk.Fee.Approx {
					p = "~"
				}
				avgRate = p + formatFloat(blk.Fee.AvgFeeRate, 0) + " sat/vB"
				feeStr = p + formatAmount(blk.Fee.TotalSats, d.Units, 2, d.Price.Value, d.Currency)
			}
			b.lines = append(b.lines, dim.Render(fmt.Sprintf("            %s tx   %s MB   %s   %s",
				formatInt(blk.TxCount), formatFloat(float64(blk.Size)/1e6, 2), avgRate, feeStr)))
		}
	}
	return b
}

func staleSuffix(d DashboardData, panel string, dim lipgloss.Style) string {
	if age, ok := d.Stale[panel]; ok {
		return "  " + dim.Render(fmt.Sprintf("⚠ stale %s", formatAge(age)))
	}
	return ""
}
