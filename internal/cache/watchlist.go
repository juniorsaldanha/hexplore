package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"go.etcd.io/bbolt"
)

var watchlistBucket = []byte("watchlist")
var watchlistWalletBucket = []byte("watchlist_wallets")

type WatchedAddress struct {
	Address string    `json:"address"`
	Label   string    `json:"label,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

// WatchedWallet is an extended public key (xpub/ypub/zpub, see
// internal/wallet) the user wants monitored — expands into many addresses
// at scan time rather than being one address itself.
type WatchedWallet struct {
	Key     string    `json:"key"`
	Label   string    `json:"label,omitempty"`
	AddedAt time.Time `json:"added_at"`
	// ScriptType is internal/wallet.ScriptType.Name() — set once the user
	// has resolved an ambiguous xpub/tpub's derivation convention, so they
	// aren't asked again on a later open. Empty for an unresolved key, or
	// one whose prefix was never ambiguous in the first place. Opaque here
	// on purpose: this package doesn't depend on internal/wallet.
	ScriptType string `json:"script_type,omitempty"`
}

// Watchlist is a small bbolt-backed store for addresses (and whole
// wallets) the user wants to keep an eye on (docs/PLAN.md §12, Phase 5) —
// persisted across restarts, unlike everything else in this package.
type Watchlist struct {
	db *bbolt.DB
}

func OpenWatchlist(path string) (*Watchlist, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("watchlist: create dir: %w", err)
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("watchlist: open %s: %w", path, err)
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(watchlistBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(watchlistWalletBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("watchlist: init: %w", err)
	}
	return &Watchlist{db: db}, nil
}

func (w *Watchlist) Close() error { return w.db.Close() }

func (w *Watchlist) Add(a WatchedAddress) error {
	if a.AddedAt.IsZero() {
		a.AddedAt = time.Now()
	}
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return w.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistBucket).Put([]byte(a.Address), data)
	})
}

func (w *Watchlist) Remove(address string) error {
	return w.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistBucket).Delete([]byte(address))
	})
}

// List returns every watched address, ordered by AddedAt (oldest first).
func (w *Watchlist) List() ([]WatchedAddress, error) {
	var out []WatchedAddress
	err := w.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistBucket).ForEach(func(_, v []byte) error {
			var a WatchedAddress
			if err := json.Unmarshal(v, &a); err != nil {
				return err
			}
			out = append(out, a)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt.Before(out[j].AddedAt) })
	return out, nil
}

func (w *Watchlist) AddWallet(wa WatchedWallet) error {
	if wa.AddedAt.IsZero() {
		wa.AddedAt = time.Now()
	}
	data, err := json.Marshal(wa)
	if err != nil {
		return err
	}
	return w.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistWalletBucket).Put([]byte(wa.Key), data)
	})
}

func (w *Watchlist) RemoveWallet(key string) error {
	return w.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistWalletBucket).Delete([]byte(key))
	})
}

// ListWallets returns every watched wallet key, ordered by AddedAt (oldest
// first).
func (w *Watchlist) ListWallets() ([]WatchedWallet, error) {
	var out []WatchedWallet
	err := w.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(watchlistWalletBucket).ForEach(func(_, v []byte) error {
			var wa WatchedWallet
			if err := json.Unmarshal(v, &wa); err != nil {
				return err
			}
			out = append(out, wa)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt.Before(out[j].AddedAt) })
	return out, nil
}
