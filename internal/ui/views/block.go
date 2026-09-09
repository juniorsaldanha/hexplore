package views

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

// blockTxPageSize matches the provider's own pagination (app.blockTxPageSize) —
// duplicated here since views can't import app (app imports views).
const blockTxPageSize = 25

type BlockData struct {
	Block  domain.Block
	Txs    []domain.Tx // current 25-tx page
	Page   int         // 0-based; matches the provider's own pagination 1:1
	Cursor int         // selected tx within the page
	Caps   domain.Capabilities

	// TreemapTxs is a (possibly sample-capped) view of the block's
	// transactions, fetched separately from Txs since it needs far more
	// than one page — see app.fetchBlockTreemap.
	TreemapTxs       []domain.Tx
	TreemapTruncated bool
	TreemapLoading   bool

	Units    Unit
	Price    float64
	Currency string

	Width, Height int
}

func Block(d BlockData, th theme.Theme) string {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(0, 1)
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	accent := lipgloss.NewStyle().Foreground(th.Accent)

	stacked := d.Width < 100
	panelWidth := d.Width - 4
	if !stacked {
		panelWidth = (d.Width - 6) / 2
	}

	// Every panel's height must stay inside what the terminal actually
	// reports (d.Height): bubbletea's redraw uses relative cursor moves
	// sized to the previous frame, and output taller than the terminal
	// breaks that math — the exact bug this once caused (the treemap
	// visually overlapping the info panels once it grew past the fold).
	// Everything below is measured from the real rendered strings
	// (lipgloss.Height), not estimated — text length varies with real
	// data (address lengths, page counts, etc.), and estimates drift.
	availableHeight := d.Height
	if availableHeight <= 0 {
		availableHeight = 40 // no WindowSizeMsg yet (e.g. a render in tests)
	}

	infoB := blockInfoBox(d, th, dim, accent, innerWidth(panelWidth))
	techB := blockTechBox(d, dim, innerWidth(panelWidth))

	var topRow string
	if !stacked {
		// Side by side never grows very tall (both boxes share the same
		// row), so it always gets to keep TECHNICAL.
		padRow(&infoB, &techB)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top,
			renderBox(infoB, panelWidth, border, title),
			renderBox(techB, panelWidth, border, title))
	} else {
		infoRendered := renderBox(infoB, panelWidth, border, title)
		techRendered := renderBox(techB, panelWidth, border, title)
		// Stacking both can, on a merely-average-width terminal (< 100
		// cols puts even an 80-col default into this branch), alone eat
		// most of a short terminal's height — reserve enough for the tx
		// list to still show at least one row before committing to both.
		const minReserveForRest = 1 + 1 + 2 + 2 + 1 + 1 // blank, txHeader, hint+composition, one (cursor) row, blank, nav
		if lipgloss.Height(infoRendered)+lipgloss.Height(techRendered)+minReserveForRest <= availableHeight {
			topRow = lipgloss.JoinVertical(lipgloss.Left, infoRendered, techRendered)
		} else {
			// TECHNICAL's version/bits/nonce/merkle matter far less than
			// actually being able to see transactions — drop it rather
			// than crowd out (or, as originally reported, corrupt) the
			// rest of the page.
			topRow = infoRendered
		}
	}

	topRowHeight := lipgloss.Height(topRow)

	txHeader := title.Render(fmt.Sprintf("Transactions (page %d of %s, %d shown)",
		d.Page+1, formatInt(max((d.Block.TxCount+blockTxPageSize-1)/blockTxPageSize, 1)), len(d.Txs)))
	// txHeader below may grow by a scroll hint and/or a composition summary
	// line — reserve for the worst case (both) rather than computing the
	// exact number, which would need to know the row budget this very
	// calculation is trying to produce.
	const txHeaderExtraLines = 2
	const footerHeight = 2 // blank + nav
	const minRowsWorthShowing = 3

	// The treemap only gets drawn if there's real room left for it *and*
	// a reasonable number of tx rows once it's accounted for — on a short
	// terminal it simply doesn't fit, and disappearing is much better than
	// crowding out (or, as reported, visually corrupting) everything else.
	fixedWithoutTreemap := topRowHeight + 1 + lipgloss.Height(txHeader) + txHeaderExtraLines + footerHeight
	roomForTreemap := availableHeight - fixedWithoutTreemap - 1 - minRowsWorthShowing

	var treemapPanel string
	fixedHeight := fixedWithoutTreemap
	const treemapChrome = 3 // border top+bottom + its own title line
	if roomForTreemap >= treemapChrome+3 {
		treemapHeight := min(roomForTreemap-treemapChrome, 12)
		treemapPanel = renderBox(actualBlockBox(d, th, dim, innerWidth(d.Width-2), treemapHeight), d.Width-2, border, title)
		fixedHeight += lipgloss.Height(treemapPanel) + 1
	}

	// Never pad this back up to a "nicer" minimum: on a terminal too short
	// even for the header, the only way to truly never exceed
	// availableHeight is to let the tx list shrink toward its floor of 1
	// row too (see scrollWindow) rather than forcing extra rows in.
	rowBudget := max(availableHeight-fixedHeight, 1)
	// The cursor row expands to 2 lines (adds an address-preview line), so
	// its budget is one row "more expensive" than the rest.
	maxRows := max(rowBudget-1, 1)

	selected := lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	start, end := scrollWindow(len(d.Txs), d.Cursor, maxRows)

	var rows []string
	var composition [4]int // indexed by enrich.TxKind — a per-page sample, not the whole block
	for i := start; i < end; i++ {
		tx := d.Txs[i]
		kind := enrich.ClassifyTx(tx)
		composition[kind]++

		var vsize float64
		if tx.Weight > 0 {
			vsize = float64(tx.Weight) / 4
		}
		var rate float64
		if vsize > 0 {
			rate = float64(tx.Fee) / vsize
		}
		rateStr := "—"
		if vsize > 0 {
			rateStr = formatFloat(rate, 1) + " sat/vB"
		}

		var line string
		if i == d.Cursor {
			plain := fmt.Sprintf("%s   %s sats  (%s)   %d in / %d out", tx.TxID, formatInt(int(tx.Fee)), rateStr, len(tx.Vin), len(tx.Vout))
			// A colour nested inside the selected style would reset styling
			// partway through the line (ANSI codes don't nest cleanly) — the
			// tag name still shows, just in the selected row's own colour.
			tag := ""
			if kind != enrich.TxNormal {
				tag = "  [" + kind.String() + "]"
			}
			// Only the focused row expands with an address preview — every
			// row doing this is what blew the page's height budget in the
			// first place.
			line = selected.Render(plain+tag) + "\n" + selected.Render(addressPreviewLine(tx))
		} else {
			rateColored := lipgloss.NewStyle().Foreground(th.FeeColor(rate)).Render(rateStr)
			line = fmt.Sprintf("%s   %s sats  (%s)   %d in / %d out", dim.Render(tx.TxID), formatInt(int(tx.Fee)), rateColored, len(tx.Vin), len(tx.Vout)) + txKindTag(kind, th)
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		rows = []string{loadingLine("transactions", th)}
	} else {
		if hidden := scrollHint(len(d.Txs), start, end, dim); hidden != "" {
			txHeader += "\n" + hidden
		}
		if summary := compositionSummary(composition, dim); summary != "" {
			txHeader += "\n" + dim.Render("this page: ") + summary
		}
	}

	nav := dim.Render("j/k select   n/p page   ↵ open tx   Esc back")

	body := []string{topRow, ""}
	if treemapPanel != "" {
		body = append(body, treemapPanel, "")
	}
	body = append(body, txHeader)
	body = append(body, rows...)
	body = append(body, "", nav)
	return strings.Join(body, "\n")
}

// scrollWindow returns the [start, end) slice of a total-length list that
// keeps cursor visible within a window of at most maxVisible items,
// anchoring the window around the cursor rather than always starting at 0.
func scrollWindow(total, cursor, maxVisible int) (start, end int) {
	if maxVisible <= 0 {
		maxVisible = 1
	}
	if total <= maxVisible {
		return 0, total
	}
	start = cursor - maxVisible/2
	start = max(start, 0)
	end = start + maxVisible
	if end > total {
		end = total
		start = max(end-maxVisible, 0)
	}
	return start, end
}

func scrollHint(total, start, end int, dim lipgloss.Style) string {
	if start == 0 && end == total {
		return ""
	}
	var parts []string
	if start > 0 {
		parts = append(parts, fmt.Sprintf("↑ %d more above", start))
	}
	if end < total {
		parts = append(parts, fmt.Sprintf("↓ %d more below", total-end))
	}
	return dim.Render(strings.Join(parts, "   "))
}

// addressPreviewLine gives a mempool.space-flavoured "first input -> first
// output" summary line for a tx row — real addresses, not just counts.
func addressPreviewLine(tx domain.Tx) string {
	in := "coinbase"
	if len(tx.Vin) > 0 {
		if tx.Vin[0].Coinbase {
			in = "coinbase"
		} else if tx.Vin[0].Address != "" {
			in = truncateMiddle(tx.Vin[0].Address, 20)
		} else {
			in = "(no address)"
		}
		if len(tx.Vin) > 1 {
			in += fmt.Sprintf(" +%d", len(tx.Vin)-1)
		}
	}
	out := "—"
	if len(tx.Vout) > 0 {
		addr := tx.Vout[0].Address
		if addr == "" {
			addr = "(OP_RETURN)"
		} else {
			addr = truncateMiddle(addr, 20)
		}
		out = fmt.Sprintf("%s %s sats", addr, formatInt(int(tx.Vout[0].Value)))
		if len(tx.Vout) > 1 {
			out += fmt.Sprintf(" +%d", len(tx.Vout)-1)
		}
	}
	return "  " + in + "  →  " + out
}

func blockInfoBox(d BlockData, th theme.Theme, dim, accent lipgloss.Style, width int) box {
	b := d.Block
	fill := float64(b.Weight) / 4_000_000
	pctSuffix := fmt.Sprintf("  %.1f%%", fill*100)
	bar := lipgloss.NewStyle().Foreground(th.Gradient(fill)).Render(components.FillBar(fill, fitBar(width, pctSuffix)))

	lines := []string{
		"hash    " + truncateMiddle(b.Hash, width-8),
		"prev    " + truncateMiddle(b.PrevHash, width-8),
		fmt.Sprintf("time    %s ago  (%s)", formatAge(time.Since(b.Time)), b.Time.Format("Mon 02 Jan 15:04")),
	}
	if b.Pool != "" {
		lines = append(lines, "pool    "+accent.Render(b.Pool))
	} else if !d.Caps.MiningPool {
		lines = append(lines, dim.Render("pool    — unavailable on this provider"))
	}
	lines = append(lines, bar+pctSuffix)

	feeLine := dim.Render("fees    — unavailable on this provider")
	if b.Fee != nil {
		prefix := ""
		if b.Fee.Approx {
			prefix = "~"
		}
		rateStyle := lipgloss.NewStyle().Foreground(th.FeeColor(b.Fee.AvgFeeRate))
		total := formatAmount(b.Fee.TotalSats, d.Units, 4, d.Price, d.Currency)
		feeLine = fmt.Sprintf("fees    %s%s total    %s%s sat/vB avg", prefix, total, prefix, rateStyle.Render(formatFloat(b.Fee.AvgFeeRate, 1)))
		if b.Fee.Burned {
			feeLine += "\n" + lipgloss.NewStyle().Foreground(th.Warn).Render("burned — coinbase claimed less than full subsidy")
		}
		if b.Fee.MinFeeRate > 0 || b.Fee.MaxFeeRate > 0 {
			feeLine += fmt.Sprintf("\nspan    %s – %s sat/vB", formatFloat(b.Fee.MinFeeRate, 1), formatFloat(b.Fee.MaxFeeRate, 1))
		}
		reward := enrich.Subsidy(b.Height) + b.Fee.TotalSats
		feeLine += fmt.Sprintf("\nreward  %s", formatAmount(reward, d.Units, 4, d.Price, d.Currency))
	}
	lines = append(lines, feeLine)

	if b.MatchRate > 0 {
		lines = append(lines, "health  "+healthBadge(b.MatchRate, th))
	}

	return box{title: fmt.Sprintf("BLOCK %s", formatInt(b.Height)), lines: lines}
}

// healthBadge mirrors mempool.space's block-template match-rate indicator —
// how closely the mined block matched what was predicted.
func healthBadge(matchRate float64, th theme.Theme) string {
	c := th.Bad
	switch {
	case matchRate >= 98:
		c = th.Good
	case matchRate >= 90:
		c = th.Warn
	}
	return lipgloss.NewStyle().Bold(true).Foreground(c).Render(formatFloat(matchRate, 1) + "%")
}

func blockTechBox(d BlockData, dim lipgloss.Style, width int) box {
	b := d.Block
	lines := []string{
		fmt.Sprintf("size       %s bytes", formatInt(int(b.Size))),
		fmt.Sprintf("weight     %s wu  (%s vB)", formatInt(int(b.Weight)), formatInt(int(b.Weight/4))),
		fmt.Sprintf("tx count   %s", formatInt(b.TxCount)),
		fmt.Sprintf("difficulty %s", formatCompact(b.Difficulty)),
		dim.Render(fmt.Sprintf("version    0x%08x", uint32(b.Version))),
		dim.Render(fmt.Sprintf("bits       0x%08x", b.Bits)),
		dim.Render(fmt.Sprintf("nonce      %s", formatInt(int(b.Nonce)))),
		dim.Render("merkle     " + truncateMiddle(b.MerkleRoot, width-11)),
	}
	return box{title: "TECHNICAL", lines: lines}
}

// actualBlockBox is a treemap of the block's real transactions — sized by
// vbyte, coloured by the same fee-tier ramp as everywhere else, plus an
// expected-vs-actual comparison line when the provider has it (native
// mempool.space data only). There's no "Expected Block" *treemap*
// counterpart: mempool.space's own site renders that from its internal
// per-transaction audit history, which isn't exposed anywhere in the
// public API — only the two aggregate numbers (expectedFees,
// expectedWeight) are, which is as far as an honest comparison can go here.
func actualBlockBox(d BlockData, th theme.Theme, dim lipgloss.Style, width, height int) box {
	if d.TreemapLoading && len(d.TreemapTxs) == 0 {
		return box{title: "ACTUAL BLOCK", lines: []string{loadingLine("treemap", th)}}
	}
	if len(d.TreemapTxs) == 0 {
		return box{title: "ACTUAL BLOCK", lines: []string{dim.Render("no transaction data")}}
	}

	items := make([]components.TreemapItem, len(d.TreemapTxs))
	for i, tx := range d.TreemapTxs {
		vsize := float64(tx.Weight) / 4
		var rate float64
		if vsize > 0 {
			rate = float64(tx.Fee) / vsize
		}
		items[i] = components.TreemapItem{Size: vsize, Color: th.FeeColor(rate)}
	}

	lines := components.Treemap(items, width, height, width*height*2)

	if cmp := expectedVsActualLine(d, th); cmp != "" {
		lines = append(lines, cmp)
	}

	title := fmt.Sprintf("ACTUAL BLOCK — %s tx", formatInt(len(d.TreemapTxs)))
	if d.TreemapTruncated {
		title += dim.Render(fmt.Sprintf(" (sample of %s of %s)", formatInt(len(d.TreemapTxs)), formatInt(d.Block.TxCount)))
	}
	return box{title: title, lines: lines}
}

// expectedVsActualLine compares what the mempool predicted for this block
// just before it was mined against what actually happened — real
// mempool.space data (extras.expectedFees/expectedWeight), not a guess.
// Empty when unavailable (esplora-derived blocks, or a block old enough
// mempool.space no longer has the prediction for).
func expectedVsActualLine(d BlockData, th theme.Theme) string {
	b := d.Block
	if b.ExpectedFeeSats <= 0 || b.ExpectedWeight <= 0 || b.Fee == nil {
		return ""
	}
	return fmt.Sprintf("expected %s, %s wu   →   actual %s %s, %s wu %s",
		formatAmount(b.ExpectedFeeSats, d.Units, 4, d.Price, d.Currency), formatInt(int(b.ExpectedWeight)),
		formatAmount(b.Fee.TotalSats, d.Units, 4, d.Price, d.Currency), deltaBadge(float64(b.Fee.TotalSats), float64(b.ExpectedFeeSats), th),
		formatInt(int(b.Weight)), deltaBadge(float64(b.Weight), float64(b.ExpectedWeight), th))
}

// deltaBadge renders "▲X.X%"/"▼X.X%" (green/red) for how actual compares to
// expected — the same arrow-and-colour language as the dashboard's 24h
// price change, reused here for consistency.
func deltaBadge(actual, expected float64, th theme.Theme) string {
	if expected == 0 {
		return ""
	}
	pct := (actual - expected) / expected * 100
	arrow, c := "▲", th.Good
	if pct < 0 {
		arrow, c = "▼", th.Bad
	}
	return lipgloss.NewStyle().Foreground(c).Render(fmt.Sprintf("%s%s%%", arrow, formatFloat(math.Abs(pct), 1)))
}

// txKindTag renders a coloured badge for anything other than an ordinary
// transaction — heuristic classification (enrich.ClassifyTx), not proof.
func txKindTag(kind enrich.TxKind, th theme.Theme) string {
	var c lipgloss.Color
	switch kind {
	case enrich.TxCoinbase:
		c = th.Good
	case enrich.TxConsolidation:
		c = th.Warn
	case enrich.TxCoinJoin:
		c = th.Accent
	default:
		return ""
	}
	return "  " + lipgloss.NewStyle().Foreground(c).Render("["+kind.String()+"]")
}

// compositionSummary counts classifications across the current page only —
// never the whole block, which would need every tx in it, not just this
// 25-tx page.
func compositionSummary(counts [4]int, dim lipgloss.Style) string {
	labels := []struct {
		kind enrich.TxKind
		name string
	}{
		{enrich.TxConsolidation, "consolidation"},
		{enrich.TxCoinJoin, "coinjoin"},
		{enrich.TxCoinbase, "coinbase"},
	}
	var parts []string
	for _, l := range labels {
		if n := counts[l.kind]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, l.name))
		}
	}
	if len(parts) == 0 {
		return dim.Render("all normal")
	}
	return strings.Join(parts, " · ")
}
