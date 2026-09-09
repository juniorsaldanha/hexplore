package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

// KeyHelp is a rendered {keys, action} pair. Kept generic (no bubbles
// dependency) so app's KeyMap is the single source of truth and this just
// renders whatever it's given.
type KeyHelp struct {
	Keys   string
	Action string
}

func Help(bindings []KeyHelp, th theme.Theme) string {
	title := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(th.Accent)

	width := 0
	for _, b := range bindings {
		width = max(width, len(b.Keys))
	}

	var rows []string
	rows = append(rows, title.Render("hexplore — keybindings"), "")
	for _, b := range bindings {
		rows = append(rows, fmt.Sprintf("  %s  %s", keyStyle.Render(fmt.Sprintf("%-*s", width, b.Keys)), b.Action))
	}
	return strings.Join(rows, "\n")
}
