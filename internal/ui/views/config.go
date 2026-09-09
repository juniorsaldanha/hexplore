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

type ConfigHost struct {
	Name   string
	Active bool
}

type ConfigData struct {
	Hosts      []ConfigHost
	Pinned     bool
	Degraded   bool
	Network    string
	Currency   string
	Theme      string
	GraphStyle components.GraphStyle
	Caps       domain.Capabilities
	TipHeight  int // 0 when unknown; hides the countdowns
}

// Config is a status view, not yet an editable form — it reports the live
// fallback-chain state and how to change it (:provider), rather than
// writing config.toml directly.
func Config(d ConfigData, th theme.Theme) string {
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(th.Dim)
	active := lipgloss.NewStyle().Bold(true).Foreground(th.Good)
	warn := lipgloss.NewStyle().Foreground(th.Warn)

	lines := []string{
		title.Render("Provider chain"),
	}
	for _, h := range d.Hosts {
		if h.Active {
			marker := active.Render("● " + h.Name)
			if d.Degraded {
				marker += "  " + warn.Render("(degraded — not the preferred host)")
			}
			lines = append(lines, marker)
		} else {
			lines = append(lines, dim.Render("○ "+h.Name))
		}
	}
	pin := "auto (failover enabled)"
	if d.Pinned {
		pin = "pinned — failover disabled until :provider auto"
	}
	lines = append(lines, "", fmt.Sprintf("mode      %s", pin))
	lines = append(lines, fmt.Sprintf("network   %s", d.Network))
	lines = append(lines, fmt.Sprintf("currency  %s", d.Currency))
	lines = append(lines, fmt.Sprintf("theme     %s", d.Theme))
	lines = append(lines, fmt.Sprintf("graph     %s  (:graph braille|block|tty to change)", d.GraphStyle))

	lines = append(lines, "", title.Render("Active host capabilities"))
	capLine := func(name string, ok bool) string {
		if ok {
			return "  " + name
		}
		return "  " + dim.Render(name+" (unavailable)")
	}
	lines = append(lines,
		capLine("block fees", d.Caps.BlockFees),
		capLine("mining pool", d.Caps.MiningPool),
		capLine("exact next block", d.Caps.NextBlockExact),
		capLine("address autocomplete", d.Caps.AddressPrefix),
		capLine("websocket", d.Caps.WebSocket),
	)

	if d.TipHeight > 0 {
		toRetarget := enrich.BlocksUntilRetarget(d.TipHeight)
		toHalving := enrich.BlocksUntilHalving(d.TipHeight)
		lines = append(lines, "", title.Render("Countdowns"),
			fmt.Sprintf("next retarget  ~%s blocks  (~%s)", formatInt(toRetarget), formatAge(enrich.EstimatedETA(toRetarget))),
			fmt.Sprintf("next halving   ~%s blocks  (~%s)", formatInt(toHalving), formatAge(enrich.EstimatedETA(toHalving))),
		)
	}

	lines = append(lines, "", dim.Render(":provider <name>  pin a host    :provider auto  re-enable failover    Esc back"))

	return strings.Join(append([]string{title.Render("Config / Provider Status"), ""}, lines...), "\n")
}
