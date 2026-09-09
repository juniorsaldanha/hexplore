package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/provider/chain"
)

func TestConfigScreenAndProviderPin(t *testing.T) {
	primary := &fakeChain{name: "primary"}
	backup := &fakeChain{name: "backup"}
	c := chain.New(primary, backup)

	m := New(c, nil, Config{Network: "mainnet", Currency: "USD", NumBlocks: 2, AssumedVSize: 140})
	m.width, m.height = 120, 40

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if len(m.stack) != 1 || m.stack[0].kind != screenConfig {
		t.Fatalf("expected config screen pushed, stack=%v", m.stack)
	}
	if out := m.View(); out == "" {
		t.Fatal("config view rendered empty")
	}
	m.pop()

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range "provider backup" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.chain.Name() != "backup" {
		t.Fatalf("expected pin to switch active host to backup, got %q", m.chain.Name())
	}
	if !c.Pinned() {
		t.Fatal("expected chain to be pinned after :provider backup")
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range "provider auto" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if c.Pinned() {
		t.Fatal("expected :provider auto to unpin")
	}
}
