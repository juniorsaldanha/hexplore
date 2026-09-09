package app

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/cache"
)

func TestWatchlistFlow(t *testing.T) {
	wl, err := cache.OpenWatchlist(filepath.Join(t.TempDir(), "watchlist.db"))
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer wl.Close()

	m, _ := newTestModel()
	m.cfg.Watchlist = wl

	// :watch bc1q... my label
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	for _, r := range "watch bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq my label" {
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	entries, err := wl.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Label != "my label" {
		t.Fatalf("expected 1 watched entry with label, got %+v", entries)
	}

	// `w` opens the watchlist screen with that entry loaded.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if len(m.stack) != 1 || m.stack[0].kind != screenWatchlist {
		t.Fatalf("expected watchlist screen pushed, stack=%v", m.stack)
	}
	if len(m.stack[0].watchlist) != 1 {
		t.Fatalf("expected 1 entry in the pushed screen, got %d", len(m.stack[0].watchlist))
	}
	if out := m.View(); out == "" {
		t.Fatal("watchlist view rendered empty")
	}

	// `d` deletes the selected entry and the store reflects it immediately.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	entries, err = wl.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected the entry to be removed, got %+v", entries)
	}
	if len(m.stack[0].watchlist) != 0 {
		t.Fatalf("expected the on-screen list to refresh too, got %+v", m.stack[0].watchlist)
	}
}

func TestWatchlistCommandsWithoutStore(t *testing.T) {
	m, _ := newTestModel() // cfg.Watchlist left nil
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if m.statusMsg == "" || len(m.stack) != 0 {
		t.Fatalf("expected no screen pushed and a status message, stack=%v msg=%q", m.stack, m.statusMsg)
	}
}

func TestWatchlistOpenFetchesLiveStatsForTiles(t *testing.T) {
	wl, err := cache.OpenWatchlist(filepath.Join(t.TempDir(), "watchlist.db"))
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer wl.Close()

	const addr = "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq" // matches fakeChain's canned address
	if err := wl.Add(cache.WatchedAddress{Address: addr, Label: "cold"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m, _ := newTestModel()
	m.cfg.Watchlist = wl

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if len(m.stack) != 1 || m.stack[0].kind != screenWatchlist {
		t.Fatalf("expected watchlist screen pushed, stack=%v", m.stack)
	}
	if !m.stack[0].watchlistLoading {
		t.Fatal("expected watchlistLoading to be true immediately after opening")
	}

	drain(t, m, cmd)

	if m.stack[0].watchlistLoading {
		t.Error("expected watchlistLoading to clear once stats resolved")
	}
	stats, ok := m.stack[0].watchlistAddrStats[addr]
	if !ok {
		t.Fatalf("expected stats for %s, got %+v", addr, m.stack[0].watchlistAddrStats)
	}
	if stats.FundedSats != 100 { // matches fakeChain's canned domain.Address{FundedSats: 100}
		t.Errorf("FundedSats = %d, want 100", stats.FundedSats)
	}
	if out := m.View(); out == "" {
		t.Fatal("watchlist view rendered empty")
	}
}
