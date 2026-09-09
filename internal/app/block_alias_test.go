package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBlockLatestAlias(t *testing.T) {
	for _, alias := range []string{"latest", "current", "tip"} {
		m, _ := newTestModel()
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
		for _, r := range "block " + alias {
			_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if len(m.stack) != 1 || m.stack[0].kind != screenBlock {
			t.Fatalf(":block %s: expected a block screen pushed, stack=%v", alias, m.stack)
		}
		drain(t, m, cmd)
		if m.stack[0].block.Hash != "hash0" {
			t.Fatalf(":block %s: expected the tip block (hash0), got %+v", alias, m.stack[0].block)
		}
	}
}

func TestBlockLatestAliasFallsBackToTipHeight(t *testing.T) {
	m, _ := newTestModel()
	m.blocks = nil
	m.tipHeight = 100 // matches fakeChain.blocks["hash0"].Height

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range "block latest" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.stack) != 1 || m.stack[0].kind != screenBlock {
		t.Fatalf("expected a block screen pushed, stack=%v", m.stack)
	}
	drain(t, m, cmd)
	if m.stack[0].block.Hash != "hash0" {
		t.Fatalf("expected block hash0 via height fallback, got %+v", m.stack[0].block)
	}
}
