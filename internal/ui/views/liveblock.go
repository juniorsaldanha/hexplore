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

// LiveBlockFilter is the "goggles" tab bar (docs: mempool.space's live
// next-block visualization) — filters which transactions are drawn, it
// doesn't just highlight them.
type LiveBlockFilter int

const (
	FilterAll LiveBlockFilter = iota
	FilterConsolidation
	FilterCoinJoin
	FilterData
)

func (f LiveBlockFilter) Next() LiveBlockFilter { return (f + 1) % 4 }

func (f LiveBlockFilter) String() string {
	switch f {
	case FilterConsolidation:
		return "Consolidation"
	case FilterCoinJoin:
		return "Coinjoin"
	case FilterData:
		return "Data"
	default:
		return "All"
	}
}

func (f LiveBlockFilter) matches(kind enrich.TxKind) bool {
	switch f {
	case FilterConsolidation:
		return kind == enrich.TxConsolidation
	case FilterCoinJoin:
		return kind == enrich.TxCoinJoin
	case FilterData:
		return kind == enrich.TxData
	default:
		return true
	}
}

type LiveBlockData struct {
	Txs           []domain.ProjectedTx
	Filter        LiveBlockFilter
	Supported     bool // false when the active provider has no live feed at all
	Width, Height int
}

// LiveBlock renders mempool.space's live "next block" transaction feed as a
// squarified treemap, sized by vbyte and coloured by the same fee-tier ramp
// as the rest of the app (docs/PLAN.md §11: colour carries meaning).
func LiveBlock(d LiveBlockData, th theme.Theme) string {
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	selectedTab := lipgloss.NewStyle().Bold(true).Foreground(th.Accent).Underline(true)

	if !d.Supported {
		return strings.Join([]string{
			title.Render("Next Block — live"),
			"",
			dim.Render("unavailable on this provider — mempool.space's /v1/ws feed is the only source for this"),
			"",
			dim.Render("Esc back"),
		}, "\n")
	}

	var tabs []string
	for _, f := range []LiveBlockFilter{FilterAll, FilterConsolidation, FilterCoinJoin, FilterData} {
		if f == d.Filter {
			tabs = append(tabs, selectedTab.Render(f.String()))
		} else {
			tabs = append(tabs, dim.Render(f.String()))
		}
	}

	var filtered []domain.ProjectedTx
	var totalVSize float64
	var totalFee int64
	var counts [5]int // indexed by enrich.TxKind
	for _, tx := range d.Txs {
		kind := enrich.ClassifyFlags(tx.Flags)
		counts[kind]++
		if !d.Filter.matches(kind) {
			continue
		}
		filtered = append(filtered, tx)
		totalVSize += tx.VSize
		totalFee += tx.FeeSats
	}

	if len(d.Txs) == 0 {
		return strings.Join([]string{
			title.Render("Next Block — live"),
			strings.Join(tabs, "   "),
			"",
			loadingLine("projected transactions", th),
		}, "\n")
	}

	items := make([]components.TreemapItem, len(filtered))
	for i, tx := range filtered {
		items[i] = components.TreemapItem{Size: tx.VSize, Color: th.FeeColor(tx.FeeRate)}
	}

	const chromeLines = 6 // title, tabs, blank, header, summary, blank before the graph
	graphHeight := max(d.Height-chromeLines, 4)
	graphWidth := max(d.Width, 10)
	maxCells := graphWidth * graphHeight * 2 // headroom: several txs can share a cell after rounding
	lines := components.Treemap(items, graphWidth, graphHeight, maxCells)

	header := fmt.Sprintf("%s shown of %s tx   %s vMB   %s sats fees",
		formatInt(len(filtered)), formatInt(len(d.Txs)), formatFloat(totalVSize/1_000_000, 2), formatInt(int(totalFee)))
	summary := fmt.Sprintf("normal %s · consolidation %s · coinjoin %s · data %s",
		formatInt(counts[enrich.TxNormal]), formatInt(counts[enrich.TxConsolidation]),
		formatInt(counts[enrich.TxCoinJoin]), formatInt(counts[enrich.TxData]))

	body := []string{
		title.Render("Next Block — live (mempool.space)"),
		strings.Join(tabs, "   "),
		header,
		dim.Render(summary),
		"",
	}
	body = append(body, lines...)
	body = append(body, "", dim.Render("f cycle filter    Esc back"))
	return strings.Join(body, "\n")
}
