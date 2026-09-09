package views

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

type TxData struct {
	Tx     domain.Tx
	Cursor int
	Width  int
	Height int

	Units    Unit
	Price    float64
	Currency string
}

func Tx(d TxData, th theme.Theme) string {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(0, 1)
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	t := d.Tx
	kind := enrich.ClassifyTx(t)

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

	headerRendered := renderBox(txHeaderBox(d, th, dim, accent, kind, innerWidth(width)), width, border, title)
	headerHeight := lipgloss.Height(headerRendered)

	nav := dim.Render("j/k scroll   Esc back")
	const footerHeight = 2 // blank + nav

	colWidth := max(width/2-1, 24)
	addrWidth := max(innerWidth(colWidth)-16, 8)

	// Each column box: border top+bottom (2), its own title row (1), and a
	// fixed totals footer row (1) — reserved before the scrollable rows get
	// whatever remains of the height budget.
	const colChrome = 4
	rowBudget := max(availableHeight-headerHeight-1-colChrome-footerHeight, 1)
	// The scroll hint, when shown, is one more line on top of the rows —
	// reserve for it rather than let it push the box past budget.
	maxRows := max(rowBudget-1, 1)

	inB := txInputsBox(t, d, th, dim, accent, addrWidth, maxRows)
	outB := txOutputsBox(t, d, dim, accent, addrWidth, maxRows)
	padRow(&inB, &outB)
	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		renderBox(inB, colWidth, border, title),
		renderBox(outB, colWidth, border, title))

	return headerRendered + "\n\n" + cols + "\n\n" + nav
}

func txHeaderBox(d TxData, th theme.Theme, dim, accent lipgloss.Style, kind enrich.TxKind, width int) box {
	t := d.Tx

	status := lipgloss.NewStyle().Foreground(th.Warn).Render("unconfirmed")
	if t.Status.Confirmed {
		status = lipgloss.NewStyle().Foreground(th.Good).Render(fmt.Sprintf("confirmed in block %s", formatInt(t.Status.BlockHeight))) +
			dim.Render(fmt.Sprintf(" (%s ago)", formatAge(time.Since(t.Status.BlockTime))))
	}
	status += txKindTag(kind, th)

	var feeRate float64
	if t.Weight > 0 {
		feeRate = float64(t.Fee) / (float64(t.Weight) / 4)
	}
	rateStyle := lipgloss.NewStyle().Foreground(th.FeeColor(feeRate))
	feeLine := fmt.Sprintf("fee     %s   %s sat/vB", formatAmount(t.Fee, d.Units, 8, d.Price, d.Currency), rateStyle.Render(formatFloat(feeRate, 1)))
	if t.RBF {
		feeLine += "  " + lipgloss.NewStyle().Foreground(th.Warn).Bold(true).Render("[RBF]")
	}

	lines := []string{
		accent.Render(truncateMiddle(t.TxID, width)),
		status,
		feeLine,
		fmt.Sprintf("size    %s bytes    weight %s wu", formatInt(int(t.Size)), formatInt(int(t.Weight))),
	}
	return box{title: "TRANSACTION", lines: lines}
}

func txInputsBox(t domain.Tx, d TxData, th theme.Theme, dim, accent lipgloss.Style, addrWidth, maxRows int) box {
	start, end := scrollWindow(len(t.Vin), d.Cursor, maxRows)
	var total int64
	for _, vin := range t.Vin {
		total += vin.Value
	}
	lines := make([]string, 0, maxRows+2)
	for _, vin := range t.Vin[start:end] {
		lines = append(lines, vinLine(vin, d, th, accent, addrWidth))
	}
	if hint := scrollHint(len(t.Vin), start, end, dim); hint != "" {
		lines = append(lines, hint)
	}
	lines = append(lines, "total   "+lipgloss.NewStyle().Bold(true).Render(formatAmount(total, d.Units, 8, d.Price, d.Currency)))
	return box{title: fmt.Sprintf("INPUTS (%d)", len(t.Vin)), lines: lines}
}

func txOutputsBox(t domain.Tx, d TxData, dim, accent lipgloss.Style, addrWidth, maxRows int) box {
	start, end := scrollWindow(len(t.Vout), d.Cursor, maxRows)
	var total int64
	for _, vout := range t.Vout {
		total += vout.Value
	}
	lines := make([]string, 0, maxRows+2)
	for _, vout := range t.Vout[start:end] {
		lines = append(lines, voutLine(vout, d, dim, accent, addrWidth))
	}
	if hint := scrollHint(len(t.Vout), start, end, dim); hint != "" {
		lines = append(lines, hint)
	}
	lines = append(lines, "total   "+lipgloss.NewStyle().Bold(true).Render(formatAmount(total, d.Units, 8, d.Price, d.Currency)))
	return box{title: fmt.Sprintf("OUTPUTS (%d)", len(t.Vout)), lines: lines}
}

func vinLine(vin domain.Vin, d TxData, th theme.Theme, accent lipgloss.Style, addrWidth int) string {
	if vin.Coinbase {
		return lipgloss.NewStyle().Foreground(th.Good).Render("coinbase (new subsidy)")
	}
	addr := accent.Render(truncateMiddle(vin.Address, addrWidth))
	return fmt.Sprintf("%s  %s", addr, formatAmount(vin.Value, d.Units, 8, d.Price, d.Currency))
}

func voutLine(vout domain.Vout, d TxData, dim, accent lipgloss.Style, addrWidth int) string {
	if vout.Address == "" {
		return dim.Italic(true).Render("(non-standard / OP_RETURN)")
	}
	addr := accent.Render(truncateMiddle(vout.Address, addrWidth))
	return fmt.Sprintf("%s  %s", addr, formatAmount(vout.Value, d.Units, 8, d.Price, d.Currency))
}

func truncateMiddle(s string, width int) string {
	if len(s) <= width {
		return s
	}
	half := (width - 3) / 2
	return s[:half] + "..." + s[len(s)-half:]
}
