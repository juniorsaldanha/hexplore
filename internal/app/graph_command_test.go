package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/ui/components"
)

func runCmdLine(t *testing.T, m *Model, line string) {
	t.Helper()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range line {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestGraphCommandSetsStyle(t *testing.T) {
	m, _ := newTestModel()
	runCmdLine(t, m, "graph tty")
	if m.graphStyle != components.GraphTTY {
		t.Fatalf("graphStyle = %v, want tty", m.graphStyle)
	}
	runCmdLine(t, m, "graph braille")
	if m.graphStyle != components.GraphBraille {
		t.Fatalf("graphStyle = %v, want braille", m.graphStyle)
	}
}

func TestGraphCommandRejectsUnknownStyle(t *testing.T) {
	m, _ := newTestModel()
	m.graphStyle = components.GraphBlock
	runCmdLine(t, m, "graph nonsense")
	if m.graphStyle != components.GraphBlock {
		t.Fatalf("graphStyle changed to %v on invalid input", m.graphStyle)
	}
	if m.statusMsg == "" {
		t.Fatal("expected a usage status message")
	}
}

func TestGraphCommandNoArgCycles(t *testing.T) {
	m, _ := newTestModel()
	m.graphStyle = components.GraphBraille
	runCmdLine(t, m, "graph")
	if m.graphStyle != components.GraphBlock {
		t.Fatalf("graphStyle = %v, want block after cycling from braille", m.graphStyle)
	}
}
