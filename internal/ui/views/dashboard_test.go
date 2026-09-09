package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func TestChangeArrowDirection(t *testing.T) {
	if arrow, _ := changeArrow(2.14, theme.Nord); arrow != "▲" {
		t.Errorf("positive change should arrow up, got %q", arrow)
	}
	if arrow, _ := changeArrow(-2.14, theme.Nord); arrow != "▼" {
		t.Errorf("negative change should arrow down, got %q", arrow)
	}
	if arrow, style := changeArrow(0, theme.Nord); arrow != "→" || style.GetForeground() != theme.Nord.Dim {
		t.Errorf("exactly-flat change should be a grey %q arrow, got %q coloured %v", "→", arrow, style.GetForeground())
	}
}

func TestPriceBoxBigTileOnlyWhenNotCompact(t *testing.T) {
	d := DashboardData{HasPrice: true, Price: domain.Price{Value: 64812.40, Change24h: 2.14}, Currency: "USD"}
	dim := lipgloss.NewStyle().Foreground(theme.Nord.Dim)

	wide := priceBox(d, theme.Nord, dim, innerWidth(60), false)
	if !strings.Contains(strings.Join(wide.lines, "\n"), "█") {
		t.Error("a wide, non-compact panel should render the big-font price tile")
	}

	narrow := priceBox(d, theme.Nord, dim, innerWidth(60), true)
	if strings.Contains(strings.Join(narrow.lines, "\n"), "█") {
		t.Error("a compact panel should fall back to the one-line price, not the big-font tile")
	}
}
