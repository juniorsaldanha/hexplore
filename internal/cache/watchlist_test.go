package cache

import (
	"path/filepath"
	"testing"
)

func TestWatchlistAddListRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	w, err := OpenWatchlist(path)
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer w.Close()

	if err := w.Add(WatchedAddress{Address: "addr1", Label: "cold storage"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := w.Add(WatchedAddress{Address: "addr2"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	list, err := w.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(list))
	}
	if list[0].Address != "addr1" || list[0].Label != "cold storage" {
		t.Errorf("unexpected first entry: %+v", list[0])
	}

	if err := w.Remove("addr1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	list, err = w.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Address != "addr2" {
		t.Fatalf("after Remove, list = %+v", list)
	}
}

func TestWatchlistPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	w, err := OpenWatchlist(path)
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	if err := w.Add(WatchedAddress{Address: "addr1"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	w2, err := OpenWatchlist(path)
	if err != nil {
		t.Fatalf("re-OpenWatchlist: %v", err)
	}
	defer w2.Close()
	list, err := w2.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Address != "addr1" {
		t.Fatalf("expected addr1 to persist across reopen, got %+v", list)
	}
}

func TestWatchlistWalletAddListRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	w, err := OpenWatchlist(path)
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer w.Close()

	if err := w.AddWallet(WatchedWallet{Key: "zpub1", Label: "cold wallet"}); err != nil {
		t.Fatalf("AddWallet: %v", err)
	}
	if err := w.AddWallet(WatchedWallet{Key: "zpub2"}); err != nil {
		t.Fatalf("AddWallet: %v", err)
	}

	list, err := w.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListWallets returned %d entries, want 2", len(list))
	}
	if list[0].Key != "zpub1" || list[0].Label != "cold wallet" {
		t.Errorf("unexpected first entry: %+v", list[0])
	}

	if err := w.RemoveWallet("zpub1"); err != nil {
		t.Fatalf("RemoveWallet: %v", err)
	}
	list, err = w.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	if len(list) != 1 || list[0].Key != "zpub2" {
		t.Fatalf("after RemoveWallet, list = %+v", list)
	}
}

// TestWatchlistWalletsAreIndependentOfAddresses guards against the two
// buckets sharing a keyspace by accident — a wallet key and an address
// watched under the same string must not collide.
func TestWatchlistWalletsAreIndependentOfAddresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlist.db")
	w, err := OpenWatchlist(path)
	if err != nil {
		t.Fatalf("OpenWatchlist: %v", err)
	}
	defer w.Close()

	const shared = "same-string"
	if err := w.Add(WatchedAddress{Address: shared}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := w.AddWallet(WatchedWallet{Key: shared}); err != nil {
		t.Fatalf("AddWallet: %v", err)
	}

	addrs, err := w.List()
	if err != nil || len(addrs) != 1 {
		t.Fatalf("List = %+v, err = %v, want 1 entry", addrs, err)
	}
	wallets, err := w.ListWallets()
	if err != nil || len(wallets) != 1 {
		t.Fatalf("ListWallets = %+v, err = %v, want 1 entry", wallets, err)
	}

	if err := w.RemoveWallet(shared); err != nil {
		t.Fatalf("RemoveWallet: %v", err)
	}
	addrs, err = w.List()
	if err != nil || len(addrs) != 1 {
		t.Fatalf("removing the wallet entry should not affect the address entry: List = %+v, err = %v", addrs, err)
	}
}
