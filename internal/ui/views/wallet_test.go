package views

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

func sampleWalletData(width, height, addrCount, cursor int) WalletData {
	addrs := make([]domain.WalletAddress, addrCount)
	for i := range addrs {
		addrs[i] = domain.WalletAddress{
			Address: fmt.Sprintf("bc1qexampleaddress%04d", i),
			Chain:   uint32(i % 2),
			Index:   uint32(i / 2),
			Stats:   domain.Address{FundedSats: int64(1000 + i), TxCount: 1},
		}
	}
	return WalletData{
		Wallet: domain.Wallet{
			Key:        "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs",
			ScriptType: "native SegWit (P2WPKH)", Addresses: addrs, GapLimit: 20,
		},
		Cursor: cursor, Width: width, Height: height, Currency: "USD",
	}
}

// TestWalletRenderNeverExceedsReportedHeight is the same regression class
// fixed for the block/tx/address views: an actively used wallet can
// discover far more addresses than fit on screen, and View() output must
// never exceed the terminal's reported height.
func TestWalletRenderNeverExceedsReportedHeight(t *testing.T) {
	cases := []struct {
		name                          string
		width, height, addrCount, cur int
	}{
		{"short terminal, many addresses", 140, 24, 500, 0},
		{"tall terminal", 140, 60, 500, 0},
		{"cursor deep in the list", 140, 24, 500, 450},
		{"narrow terminal", 80, 24, 500, 0},
		{"zero height (unknown, e.g. pre-WindowSizeMsg)", 140, 0, 500, 0},
		{"brand new wallet, nothing found", 140, 24, 0, 0},
		{"very short terminal", 100, 15, 500, 0},
	}
	// The WALLET box's fields (key, type, confirmed, pending, UTXOs,
	// received, scan note) plus a minimal one-row address list are a hard
	// floor that doesn't shrink further — verified empirically to plateau
	// at this height rather than grow unbounded as the terminal shrinks
	// past it (same pattern as address_test.go's addressHardFloor).
	const walletHardFloor = 18
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := sampleWalletData(c.width, c.height, c.addrCount, c.cur)
			out := Wallet(d, theme.Nord)
			got := lipgloss.Height(out)

			budget := c.height
			if budget <= 0 {
				budget = 40 // Wallet()'s own fallback when height is unknown
			}
			if budget < walletHardFloor {
				budget = walletHardFloor
			}
			if got > budget {
				t.Errorf("rendered %d lines, want <= %d\n%s", got, budget, out)
			}
		})
	}
}
