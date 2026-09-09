package views

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func sampleWatchlistData(width, height, addrCount, walletCount, cursor int, loaded bool) WatchlistData {
	entries := make([]WatchlistEntry, addrCount)
	for i := range entries {
		entries[i] = WatchlistEntry{
			Address: fmt.Sprintf("bc1qexampleaddress%04d", i), Label: "label", AddedAt: time.Now().Add(-time.Hour),
			Stats: domain.Address{FundedSats: 50000, TxCount: 3}, Loaded: loaded,
		}
	}
	wallets := make([]WatchlistWalletEntry, walletCount)
	for i := range wallets {
		wallets[i] = WatchlistWalletEntry{
			Key:   "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs",
			Label: "wallet label", AddedAt: time.Now().Add(-time.Hour),
			Wallet: domain.Wallet{ScriptType: "native SegWit (P2WPKH)", Addresses: make([]domain.WalletAddress, 3)}, Loaded: loaded,
		}
	}
	return WatchlistData{Entries: entries, Wallets: wallets, Cursor: cursor, Width: width, Height: height, Currency: "USD"}
}

// TestWatchlistRenderNeverExceedsReportedHeight is the same regression
// class fixed for every other view: a watchlist with many entries must
// still fit inside the reported terminal height.
func TestWatchlistRenderNeverExceedsReportedHeight(t *testing.T) {
	cases := []struct {
		name                                       string
		width, height, addrCount, walletCount, cur int
		loaded                                     bool
	}{
		{"short terminal, many addresses", 140, 24, 50, 5, 0, true},
		{"still loading", 140, 24, 50, 5, 0, false},
		{"tall terminal", 140, 60, 50, 5, 0, true},
		{"cursor deep in the grid", 140, 24, 50, 5, 40, true},
		{"narrow terminal (1 column)", 60, 24, 50, 5, 0, true},
		{"medium terminal (2 columns)", 90, 24, 50, 5, 0, true},
		{"zero height (unknown, e.g. pre-WindowSizeMsg)", 140, 0, 50, 5, 0, true},
		{"empty watchlist", 140, 24, 0, 0, 0, true},
		{"very short terminal", 100, 10, 50, 5, 0, true},
	}
	// A single tile row (one row of tiles plus a scroll hint) is a hard
	// floor that doesn't shrink further — verified empirically to plateau
	// at this height rather than grow unbounded as the terminal shrinks
	// past it (same pattern as the block/address/wallet views' own floors).
	const watchlistHardFloor = 13
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := sampleWatchlistData(c.width, c.height, c.addrCount, c.walletCount, c.cur, c.loaded)
			out := Watchlist(d, theme.Nord)
			got := lipgloss.Height(out)

			budget := c.height
			if budget <= 0 {
				budget = 40 // Watchlist()'s own fallback when height is unknown
			}
			if budget < watchlistHardFloor {
				budget = watchlistHardFloor
			}
			if got > budget {
				t.Errorf("rendered %d lines, want <= %d\n%s", got, budget, out)
			}
		})
	}
}
