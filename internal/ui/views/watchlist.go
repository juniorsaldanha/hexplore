package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

type WatchlistEntry struct {
	Address string
	Label   string
	AddedAt time.Time
	// Stats is the address's live balance/tx data — zero value until
	// Loaded, fetched separately from the (instant, store-only) list.
	Stats  domain.Address
	Loaded bool
}

// WatchlistWalletEntry is a watched xpub/ypub/zpub — expands into many
// addresses at scan time rather than being one address itself, see
// internal/wallet.
type WatchlistWalletEntry struct {
	Key     string
	Label   string
	AddedAt time.Time
	// Wallet is the full gap-limit scan result — zero value until Loaded.
	Wallet domain.Wallet
	Loaded bool
}

// WatchlistData's Cursor indexes a single combined list: addresses first
// (0..len(Entries)-1), then wallets — one flat, up/down-navigable list
// rather than two independently-focused ones, laid out as a tile grid
// (dashboard-style) rather than a plain list.
type WatchlistData struct {
	Entries []WatchlistEntry
	Wallets []WatchlistWalletEntry
	Cursor  int
	Width   int
	Height  int

	Units    Unit
	Price    float64
	Currency string
}

// watchlistTileLines is fixed across every tile (address or wallet) so the
// grid never needs per-row height padding — every tile is the same shape.
const watchlistTileLines = 4

func Watchlist(d WatchlistData, th theme.Theme) string {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(th.Border).Padding(0, 1)
	selectedBorder := border.BorderForeground(th.Accent)
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)

	width := d.Width
	if width <= 0 {
		width = 100
	}
	availableHeight := d.Height
	if availableHeight <= 0 {
		availableHeight = 40
	}

	total := len(d.Entries) + len(d.Wallets)
	nav := dim.Render(":watch <addr|xpub> [label]   :unwatch <addr|xpub>   d delete selected   ↵ open   Esc back")

	header := title.Render("Watchlist")
	if total == 0 {
		msg := dim.Render("nothing watched yet — :watch <address|xpub> [label] to add one")
		return header + "\n\n" + msg + "\n\n" + nav
	}

	cols := 1
	switch {
	case width >= 110:
		cols = 3
	case width >= 70:
		cols = 2
	}
	tileWidth := max((width-(cols-1)*2)/cols, 24)

	tiles := make([]string, total)
	for i := range d.Entries {
		b := selectedBorder
		if i != d.Cursor {
			b = border
		}
		tiles[i] = renderBox(addressTile(d, i, dim, th), tileWidth, b, title)
	}
	for i, wa := range d.Wallets {
		idx := len(d.Entries) + i
		b := selectedBorder
		if idx != d.Cursor {
			b = border
		}
		tiles[idx] = renderBox(walletTile(d, wa, dim, th), tileWidth, b, title)
	}

	// Measured, not guessed — a fixed content-line count per tile means
	// every tile box is exactly watchlistTileLines+3 (border top+bottom,
	// title) lines tall regardless of data, so row math is exact rather
	// than dependent on lipgloss.Height of real content.
	const tileHeight = watchlistTileLines + 3
	const footerHeight = 2 // blank + nav

	totalRows := (total + cols - 1) / cols
	cursorRow := d.Cursor / cols
	// Reserve for a scroll hint (a blank line + the hint text, 2 lines)
	// on top of the tile rows themselves — worst case, so a truncated
	// grid never exceeds budget even when the hint is shown.
	rowBudget := max(availableHeight-lipgloss.Height(header)-4, 1)
	maxRowsVisible := max(rowBudget/(tileHeight+1), 1)
	startRow, endRow := scrollWindow(totalRows, cursorRow, maxRowsVisible)

	var rows []string
	for r := startRow; r < endRow; r++ {
		rowStart := r * cols
		rowEnd := min(rowStart+cols, total)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tiles[rowStart:rowEnd]...))
	}
	if hint := scrollHint(totalRows, startRow, endRow, dim); hint != "" {
		rows = append(rows, hint)
	}

	return header + "\n\n" + strings.Join(rows, "\n\n") + "\n\n" + nav
}

func addressTile(d WatchlistData, i int, dim lipgloss.Style, th theme.Theme) box {
	e := d.Entries[i]
	label := e.Label
	if label == "" {
		label = dim.Render("(no label)")
	}

	var lines []string
	if !e.Loaded {
		lines = []string{label, loadingLine("balance", th), "", dim.Render("added " + formatAge(time.Since(e.AddedAt)) + " ago")}
	} else {
		pending := dim.Render("no pending activity")
		if e.Stats.MempoolTxCount > 0 {
			delta := e.Stats.MempoolFundedSats - e.Stats.MempoolSpentSats
			sign, style := "", lipgloss.NewStyle().Foreground(th.Bad)
			if delta >= 0 {
				sign, style = "+", lipgloss.NewStyle().Foreground(th.Good)
			}
			pending = style.Render(sign + formatAmount(delta, d.Units, 8, d.Price, d.Currency))
		}
		lines = []string{
			label,
			formatAmount(e.Stats.BalanceSats(), d.Units, 8, d.Price, d.Currency),
			pending,
			dim.Render(fmt.Sprintf("%s tx  ·  added %s ago", formatInt(e.Stats.TxCount), formatAge(time.Since(e.AddedAt)))),
		}
	}
	return box{title: truncateMiddle(e.Address, 30), lines: lines}
}

func walletTile(d WatchlistData, wa WatchlistWalletEntry, dim lipgloss.Style, th theme.Theme) box {
	label := wa.Label
	if label == "" {
		label = dim.Render("(no label)")
	}
	tileTitle := truncateMiddle(wa.Key, 26) + "  " + lipgloss.NewStyle().Foreground(th.Warn).Render("WALLET")

	var lines []string
	if !wa.Loaded {
		lines = []string{label, loadingLine("wallet scan", th), "", dim.Render("added " + formatAge(time.Since(wa.AddedAt)) + " ago")}
	} else {
		w := wa.Wallet
		scriptType := w.ScriptType
		if scriptType == "" {
			scriptType = dim.Render("unknown type")
		}
		truncNote := ""
		if w.Truncated {
			truncNote = "  " + lipgloss.NewStyle().Foreground(th.Warn).Render("(capped)")
		}
		lines = []string{
			label,
			dim.Render(scriptType),
			fmt.Sprintf("%s  ·  %s addr used%s", formatAmount(w.BalanceSats(), d.Units, 8, d.Price, d.Currency), formatInt(len(w.Addresses)), truncNote),
			dim.Render("added " + formatAge(time.Since(wa.AddedAt)) + " ago"),
		}
	}
	return box{title: tileTitle, lines: lines}
}
