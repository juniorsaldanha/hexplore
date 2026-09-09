package views

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

type WalletData struct {
	Wallet domain.Wallet
	Cursor int
	Width  int
	Height int

	Units    Unit
	Price    float64
	Currency string
}

func Wallet(d WalletData, th theme.Theme) string {
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

	headerRendered := renderBox(walletInfoBox(d, th, dim, accent, innerWidth(width)), width, border, title)
	headerHeight := lipgloss.Height(headerRendered)

	nav := dim.Render("j/k select   ↵ open address   Esc back")
	const footerHeight = 2 // blank + nav
	const listChrome = 3   // border top+bottom + title

	rowBudget := max(availableHeight-headerHeight-1-listChrome-footerHeight, 1)
	// The scroll hint, when shown, is one more line on top of the rows —
	// reserve for it rather than let it push the box past budget.
	maxRows := max(rowBudget-1, 1)

	addrs := d.Wallet.Addresses
	start, end := scrollWindow(len(addrs), d.Cursor, maxRows)
	var rows []string
	for i := start; i < end; i++ {
		wa := addrs[i]
		chainLabel := "receive"
		if wa.Chain == 1 {
			chainLabel = "change "
		}
		line := fmt.Sprintf("%s/%-3d  %s   %s", chainLabel, wa.Index, truncateMiddle(wa.Address, 42),
			formatAmount(wa.Stats.BalanceSats(), d.Units, 8, d.Price, d.Currency))
		if i == d.Cursor {
			line = selected.Render(line)
		}
		rows = append(rows, line)
	}
	if len(rows) == 0 {
		rows = []string{dim.Render("no used addresses found")}
	}
	if hint := scrollHint(len(addrs), start, end, dim); hint != "" {
		rows = append(rows, hint)
	}
	listRendered := renderBox(box{title: fmt.Sprintf("ADDRESSES (%d)", len(addrs)), lines: rows}, width, border, title)

	return headerRendered + "\n\n" + listRendered + "\n\n" + nav
}

func walletInfoBox(d WalletData, th theme.Theme, dim, accent lipgloss.Style, width int) box {
	w := d.Wallet
	lines := []string{accent.Render(truncateMiddle(w.Key, width))}

	if w.ScriptType != "" {
		lines = append(lines, "type       "+lipgloss.NewStyle().Foreground(th.Accent).Render(w.ScriptType))
	}

	pendingLine := dim.Render("no pending activity")
	if pending := w.PendingBalanceSats(); pending != 0 {
		sign, style := "", lipgloss.NewStyle().Foreground(th.Bad)
		if pending > 0 {
			sign, style = "+", lipgloss.NewStyle().Foreground(th.Good)
		}
		pendingLine = style.Render(sign + formatAmount(pending, d.Units, 8, d.Price, d.Currency))
	}

	lines = append(lines,
		fmt.Sprintf("confirmed  %s", formatAmount(w.BalanceSats(), d.Units, 8, d.Price, d.Currency)),
		fmt.Sprintf("pending    %s", pendingLine),
		fmt.Sprintf("UTXOs      %s confirmed  ·  %s pending", formatInt(w.ConfirmedUTXOs()), formatInt(w.PendingUTXOs())),
		fmt.Sprintf("received   %s total", formatAmount(w.ReceivedSats(), d.Units, 8, d.Price, d.Currency)),
	)

	scanNote := fmt.Sprintf("%s addresses used  ·  gap limit %s", formatInt(len(w.Addresses)), formatInt(w.GapLimit))
	if w.Truncated {
		scanNote += "  " + lipgloss.NewStyle().Foreground(th.Warn).Render("(hit the scan cap — some activity may be missing)")
	}
	lines = append(lines, dim.Render(scanNote))

	return box{title: "WALLET", lines: lines}
}
