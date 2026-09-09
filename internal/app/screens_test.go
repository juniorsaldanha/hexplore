package app

import (
	"fmt"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/wallet"
)

func TestNetEffect(t *testing.T) {
	const addr = "bc1qtestaddr"
	cases := []struct {
		name string
		tx   domain.Tx
		want int64
	}{
		{"pure receive", domain.Tx{Vout: []domain.Vout{{Address: addr, Value: 500}}}, 500},
		{
			"spend with change", domain.Tx{
				Vin:  []domain.Vin{{Address: addr, Value: 200}},
				Vout: []domain.Vout{{Address: addr, Value: 100}, {Address: "someone-else", Value: 90}},
			}, -100,
		},
		{
			"unrelated tx", domain.Tx{
				Vin:  []domain.Vin{{Address: "someone-else", Value: 200}},
				Vout: []domain.Vout{{Address: "another", Value: 190}},
			}, 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := netEffect(c.tx, addr); got != c.want {
				t.Errorf("netEffect() = %d, want %d", got, c.want)
			}
		})
	}
}

// TestFetchAddressHistoryReconstructsRunningBalance is the table test the
// running-balance reconstruction earns for being the highest-risk pure
// logic in this feature — same spirit as enrich.CoinbaseFees.
func TestFetchAddressHistoryReconstructsRunningBalance(t *testing.T) {
	const addr = "bc1qtestaddr"
	// Newest-first, exactly as AddressTxs returns them: tx3 (newest) pays
	// addr +500; tx2 nets addr -100 (spends 200, gets 100 change); tx1
	// (oldest) pays addr +300. Working balance is 1000 right now.
	tx3 := domain.Tx{TxID: "tx3", Vout: []domain.Vout{{Address: addr, Value: 500}}}
	tx2 := domain.Tx{TxID: "tx2", Vin: []domain.Vin{{Address: addr, Value: 200}}, Vout: []domain.Vout{{Address: addr, Value: 100}}}
	tx1 := domain.Tx{TxID: "tx1", Vout: []domain.Vout{{Address: addr, Value: 300}}}

	fc := &fakeChain{addrTxByCursor: map[string][]domain.Tx{
		"": {tx3, tx2, tx1},
	}}

	msg := fetchAddressHistory(t.Context(), fc, addr, 1000)().(addressHistoryMsg)

	want := []float64{600, 500, 1000} // oldest -> newest: after tx1, after tx2, after tx3 (= now)
	if len(msg.series) != len(want) {
		t.Fatalf("series = %v, want %v", msg.series, want)
	}
	for i := range want {
		if msg.series[i] != want[i] {
			t.Errorf("series[%d] = %v, want %v (full: %v)", i, msg.series[i], want[i], msg.series)
		}
	}
	if msg.truncated {
		t.Error("a single short page should never be reported as truncated")
	}
}

// TestFetchAddressHistoryPopulatesFeaturesFromTheSameSample confirms
// fetchAddressHistory wires its tx sample into enrich.AddressFeatures
// rather than computing the series in isolation.
func TestFetchAddressHistoryPopulatesFeaturesFromTheSameSample(t *testing.T) {
	const addr = "bc1qtestaddr"
	fc := &fakeChain{addrTxByCursor: map[string][]domain.Tx{
		"": {{TxID: "tx1", RBF: true}},
	}}

	msg := fetchAddressHistory(t.Context(), fc, addr, 0)().(addressHistoryMsg)

	want := []string{"RBF"}
	if len(msg.features) != len(want) || msg.features[0] != want[0] {
		t.Errorf("features = %v, want %v", msg.features, want)
	}
}

func TestFetchAddressHistoryStopsAtSamplePagesCapAndFlagsTruncated(t *testing.T) {
	const addr = "bc1qtestaddr"
	byCursor := map[string][]domain.Tx{}
	cursor := ""
	for p := range addressHistorySamplePages {
		page := make([]domain.Tx, txPageSize)
		for i := range page {
			page[i] = domain.Tx{TxID: fmt.Sprintf("p%d-t%d", p, i)}
		}
		byCursor[cursor] = page
		cursor = page[len(page)-1].TxID
	}
	fc := &fakeChain{addrTxByCursor: byCursor}

	msg := fetchAddressHistory(t.Context(), fc, addr, 0)().(addressHistoryMsg)

	if !msg.truncated {
		t.Error("expected truncated=true after exhausting addressHistorySamplePages full pages")
	}
	if want := addressHistorySamplePages * txPageSize; len(msg.series) != want {
		t.Errorf("series length = %d, want %d", len(msg.series), want)
	}
}

func TestFetchWalletScanFindsUsedAddressesAndRespectsGapLimit(t *testing.T) {
	const zpub = "zpub6rFR7y4Q2AijBEqTUquhVz398htDFrtymD9xYYfG1m4wAcvPhXNfE3EfH1r1ADqtfSdVCToUG868RvUUkgDKf31mGDtKsAYz2oz2AGutZYs"
	key, err := wallet.Parse(zpub)
	if err != nil {
		t.Fatal(err)
	}

	addrs := map[string]domain.Address{}
	// Populate every address the scan could touch (comfortably past two
	// gap-limit batches per chain) as explicitly unused — matching a real
	// provider's behaviour of returning zero stats, never an error, for a
	// syntactically valid but never-funded address.
	for chainIdx := uint32(0); chainIdx < 2; chainIdx++ {
		for index := uint32(0); index < walletGapLimit*3; index++ {
			addr, err := key.Address(chainIdx, index)
			if err != nil {
				t.Fatal(err)
			}
			addrs[addr] = domain.Address{Address: addr}
		}
	}
	// Fund three receive addresses within the first batch — real activity
	// that must not stop the scan early, and a change chain left entirely
	// unused, which should stop after just one batch.
	usedAddrs := map[string]bool{}
	for _, index := range []uint32{0, 1, 2} {
		addr, _ := key.Address(0, index)
		addrs[addr] = domain.Address{Address: addr, FundedSats: 50000, TxCount: 1}
		usedAddrs[addr] = true
	}

	fc := &fakeChain{addrs: addrs}
	msg := fetchWalletScan(t.Context(), fc, zpub)().(walletScanMsg)

	if msg.err != nil {
		t.Fatalf("fetchWalletScan error: %v", msg.err)
	}
	if len(msg.wallet.Addresses) != 3 {
		t.Fatalf("found %d used addresses, want 3: %+v", len(msg.wallet.Addresses), msg.wallet.Addresses)
	}
	for _, wa := range msg.wallet.Addresses {
		if !usedAddrs[wa.Address] {
			t.Errorf("unexpected discovered address %s", wa.Address)
		}
		if wa.Chain != 0 {
			t.Errorf("address %s has chain %d, want 0 (no change addresses were funded)", wa.Address, wa.Chain)
		}
	}
	if msg.wallet.Truncated {
		t.Error("should not be truncated — well within walletMaxScanPerChain")
	}
	if got := msg.wallet.BalanceSats(); got != 150000 {
		t.Errorf("aggregate BalanceSats() = %d, want 150000", got)
	}
}

func TestFetchWalletScanRejectsPrivateKey(t *testing.T) {
	const zprv = "zprvAdG4iTXWBoARxkkzNpNh8r6Qag3irQB8PzEMkAFeTRXxHpbF9z4QgEvBRmfvqWvGp42t42nvgGpNgYSJA9iefm1yYNZKEm7z6qUWCroSQnE"
	fc := &fakeChain{}
	msg := fetchWalletScan(t.Context(), fc, zprv)().(walletScanMsg)
	if msg.err != wallet.ErrPrivateKey {
		t.Fatalf("error = %v, want wallet.ErrPrivateKey", msg.err)
	}
}
