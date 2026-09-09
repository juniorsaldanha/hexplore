package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

type AddressData struct {
	Address domain.Address
	Txs     []domain.Tx
	Cursor  int
	Width   int
	Height  int

	Units    Unit
	Price    float64
	Currency string

	GraphStyle components.GraphStyle
	// History is a running balance series in sats, chronological (oldest
	// -> newest), reconstructed by app.fetchAddressHistory from a bounded
	// sample of the address's own tx history — see that function's
	// comment for why it's a sample, not the full history.
	History          []float64
	HistoryTruncated bool
	HistoryLoading   bool

	// Features is enrich.AddressFeatures — independent, co-occurring
	// traits (SegWit, Taproot, RBF, CoinJoin, ...), not a single exclusive
	// "type": ClassifyAddress's script type is the one thing an address
	// has exactly one of, rendered separately below.
	Features []string
}

func Address(d AddressData, th theme.Theme) string {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(0, 1)
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	selected := lipgloss.NewStyle().Bold(true).Foreground(th.Accent)

	width := d.Width
	if width <= 0 {
		width = 100
	}
	// Measured, not guessed — see block.go's history with this exact class
	// of bug (View() output taller than the terminal breaks bubbletea's
	// redraw).
	availableHeight := d.Height
	if availableHeight <= 0 {
		availableHeight = 40
	}

	infoRendered := renderBox(addressInfoBox(d, th, dim, accent, innerWidth(width)), width, border, title)
	infoHeight := lipgloss.Height(infoRendered)

	nav := dim.Render("j/k select   n more (older)   ↵ open tx   Esc back")
	const footerHeight = 2 // blank + nav

	// The balance-history graph is optional richness, same philosophy as
	// the block view's treemap: only draw it if there's still enough room
	// left over for the tx list to show a reasonable number of rows.
	const graphHeight = 3
	const historyChrome = 4 // border top+bottom + title + low/high line
	const minRowsWorthShowing = 3
	const listChrome = 3 // border top+bottom + title

	roomForHistory := availableHeight - infoHeight - 1 - footerHeight - listChrome - minRowsWorthShowing - 1
	var historyRendered string
	if roomForHistory >= historyChrome+graphHeight {
		historyRendered = renderBox(addressHistoryBox(d, th, dim, innerWidth(width), graphHeight), width, border, title)
	}

	fixedHeight := infoHeight + 1 + footerHeight
	if historyRendered != "" {
		fixedHeight += lipgloss.Height(historyRendered) + 1
	}
	rowBudget := max(availableHeight-fixedHeight-listChrome, 1)
	// The scroll hint, when shown, is one more line on top of the rows —
	// reserve for it rather than let it push the box past budget.
	maxRows := max(rowBudget-1, 1)

	start, end := scrollWindow(len(d.Txs), d.Cursor, maxRows)
	var rows []string
	for i := start; i < end; i++ {
		tx := d.Txs[i]
		conf := dim.Render("unconfirmed")
		if tx.Status.Confirmed {
			conf = "block " + formatInt(tx.Status.BlockHeight)
		}
		line := fmt.Sprintf("%s   %s   fee %s", truncateMiddle(tx.TxID, 20), conf, formatAmount(tx.Fee, d.Units, 8, d.Price, d.Currency))
		if i == d.Cursor {
			line = selected.Render(line)
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		rows = []string{loadingLine("history", th)}
	}
	if hint := scrollHint(len(d.Txs), start, end, dim); hint != "" {
		rows = append(rows, hint)
	}
	listRendered := renderBox(box{title: fmt.Sprintf("HISTORY (%s tx)", formatInt(d.Address.TxCount)), lines: rows}, width, border, title)

	parts := []string{infoRendered}
	if historyRendered != "" {
		parts = append(parts, historyRendered)
	}
	parts = append(parts, listRendered)

	return strings.Join(parts, "\n\n") + "\n\n" + nav
}

func addressInfoBox(d AddressData, th theme.Theme, dim, accent lipgloss.Style, width int) box {
	a := d.Address
	lines := []string{accent.Render(truncateMiddle(a.Address, width))}

	if kind := enrich.ClassifyAddress(a.Address); kind != "" {
		lines = append(lines, "type       "+lipgloss.NewStyle().Foreground(th.Accent).Render(kind))
	}
	if badges := featureBadges(d.Features, th); badges != "" {
		lines = append(lines, "features   "+badges)
	}

	pendingLine := dim.Render("no pending activity")
	if a.MempoolTxCount > 0 {
		delta := a.MempoolFundedSats - a.MempoolSpentSats
		sign, style := "", lipgloss.NewStyle().Foreground(th.Bad)
		if delta >= 0 {
			sign, style = "+", lipgloss.NewStyle().Foreground(th.Good)
		}
		pendingLine = style.Render(sign+formatAmount(delta, d.Units, 8, d.Price, d.Currency)) +
			dim.Render(fmt.Sprintf(" (%s unconfirmed tx)", formatInt(a.MempoolTxCount)))
	}

	lines = append(lines,
		fmt.Sprintf("confirmed  %s", formatAmount(a.BalanceSats(), d.Units, 8, d.Price, d.Currency)),
		fmt.Sprintf("pending    %s", pendingLine),
		fmt.Sprintf("UTXOs      %s confirmed  ·  %s pending", formatInt(a.ConfirmedUTXOs()), formatInt(a.MempoolFundedTxoCount)),
		fmt.Sprintf("received   %s total", formatAmount(a.FundedSats, d.Units, 8, d.Price, d.Currency)),
		dim.Render(fmt.Sprintf("%s total tx", formatInt(a.TxCount))),
	)
	return box{title: "ADDRESS", lines: lines}
}

// featureBadges colours enrich.AddressFeatures — these are independent,
// co-occurring traits (unlike the single-choice script type above), so
// they render as a joined list of separately-coloured badges rather than
// one value.
func featureBadges(feats []string, th theme.Theme) string {
	if len(feats) == 0 {
		return ""
	}
	colorFor := func(f string) lipgloss.Color {
		switch f {
		case "SegWit", "Taproot":
			return th.Accent
		case "RBF", "CoinJoin", "Consolidation":
			return th.Warn
		case "Coinbase Payouts":
			return th.Good
		default:
			return th.Fg
		}
	}
	parts := make([]string, len(feats))
	for i, f := range feats {
		parts[i] = lipgloss.NewStyle().Foreground(colorFor(f)).Render(f)
	}
	return strings.Join(parts, "  ·  ")
}

// addressHistoryBox charts the running-balance sample from
// app.fetchAddressHistory in the caller's active unit (BTC/sat/fiat) — the
// same global toggle (`u`) every other view honours, so switching it here
// switches the graph's axis, not just its labels.
func addressHistoryBox(d AddressData, th theme.Theme, dim lipgloss.Style, width, height int) box {
	title := "BALANCE HISTORY"
	if d.HistoryLoading && len(d.History) == 0 {
		return box{title: title, lines: []string{loadingLine("balance history", th)}}
	}
	if d.Units == UnitFiat && d.Price <= 0 {
		return box{title: title, lines: []string{dim.Render("fiat unavailable — no price data")}}
	}
	if len(d.History) < 2 {
		return box{title: title, lines: []string{dim.Render("not enough history yet")}}
	}

	series := make([]float64, len(d.History))
	for i, sats := range d.History {
		series[i] = satsToUnit(sats, d.Units, d.Price)
	}
	lines := components.Graph(series, width, height, d.GraphStyle, th.Gradient)
	lo, hi := minMax(series)
	decimals := unitDecimals(d.Units)
	lines = append(lines, dim.Render(fmt.Sprintf("low %s   high %s", formatFloat(lo, decimals), formatFloat(hi, decimals))))

	if d.HistoryTruncated {
		title += dim.Render(fmt.Sprintf(" (last %s tx)", formatInt(len(d.History))))
	}
	return box{title: title, lines: lines}
}

// satsToUnit is formatAmount's numeric counterpart — a raw value for
// graphing rather than a formatted string with a unit suffix.
func satsToUnit(sats float64, unit Unit, price float64) float64 {
	switch unit {
	case UnitSat:
		return sats
	case UnitFiat:
		return sats / 1e8 * price
	default:
		return sats / 1e8
	}
}

func unitDecimals(unit Unit) int {
	switch unit {
	case UnitSat:
		return 0
	case UnitFiat:
		return 2
	default:
		return 4
	}
}
