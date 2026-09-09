package app

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/cache"
	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/enrich"
	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/ui/views"
	"github.com/juniorsaldanha/hexplore/internal/wallet"
)

type screenKind int

const (
	screenBlock screenKind = iota
	screenTx
	screenAddress
	screenHelp
	screenConfig
	screenWatchlist
	screenLiveBlock
	screenWallet
)

// screen is one entry in the navigation stack (docs/PLAN.md §8, Esc pops).
// Only the fields matching kind are populated.
type screen struct {
	kind screenKind

	block     domain.Block
	blockTxs  []domain.Tx
	blockPage int
	loading   bool

	tx domain.Tx

	address    domain.Address
	addressTxs []domain.Tx

	addressHistory          []float64
	addressHistoryTruncated bool
	addressHistoryLoading   bool
	addressFeatures         []string

	watchlist        []cache.WatchedAddress
	watchlistWallets []cache.WatchedWallet
	// watchlistAddrStats/watchlistWalletStats key by WatchedAddress.Address
	// / WatchedWallet.Key — enrichment for the tile view, fetched
	// separately from the (instant, store-only) list above.
	watchlistAddrStats   map[string]domain.Address
	watchlistWalletStats map[string]domain.Wallet
	watchlistLoading     bool

	wallet domain.Wallet

	liveBlockTxs    []domain.ProjectedTx
	liveBlockFilter views.LiveBlockFilter

	blockTreemapTxs       []domain.Tx
	blockTreemapTruncated bool
	blockTreemapLoading   bool

	cursor int
	err    error
}

const txPageSize = 25

type blockLoadedMsg struct {
	block domain.Block
	txs   []domain.Tx
	err   error
}

type blockPageMsg struct {
	txs  []domain.Tx
	page int
	err  error
}

type txLoadedMsg struct {
	tx  domain.Tx
	err error
}

type addressLoadedMsg struct {
	address domain.Address
	txs     []domain.Tx
	err     error
}

func fetchBlockByHash(ctx context.Context, chain provider.ChainProvider, hash string) tea.Cmd {
	return func() tea.Msg {
		b, err := chain.BlockByHash(ctx, hash)
		if err != nil {
			return blockLoadedMsg{err: err}
		}
		txs, err := chain.BlockTxs(ctx, hash, 0)
		return blockLoadedMsg{block: b, txs: txs, err: err}
	}
}

func fetchBlockByHeight(ctx context.Context, chain provider.ChainProvider, height int) tea.Cmd {
	return func() tea.Msg {
		b, err := chain.BlockByHeight(ctx, height)
		if err != nil {
			return blockLoadedMsg{err: err}
		}
		txs, err := chain.BlockTxs(ctx, b.Hash, 0)
		return blockLoadedMsg{block: b, txs: txs, err: err}
	}
}

func fetchBlockPage(ctx context.Context, chain provider.ChainProvider, hash string, page int) tea.Cmd {
	return func() tea.Msg {
		txs, err := chain.BlockTxs(ctx, hash, page*txPageSize)
		return blockPageMsg{txs: txs, page: page, err: err}
	}
}

func fetchTx(ctx context.Context, chain provider.ChainProvider, txid string) tea.Cmd {
	return func() tea.Msg {
		t, err := chain.Tx(ctx, txid)
		return txLoadedMsg{tx: t, err: err}
	}
}

// fetchAmbiguousHash resolves the 64-hex ambiguity (docs/PLAN.md §7): try
// tx first, fall back to block on error.
func fetchAmbiguousHash(ctx context.Context, chain provider.ChainProvider, hash string) tea.Cmd {
	return func() tea.Msg {
		t, err := chain.Tx(ctx, hash)
		if err == nil {
			return txLoadedMsg{tx: t}
		}
		b, berr := chain.BlockByHash(ctx, hash)
		if berr != nil {
			return txLoadedMsg{err: err}
		}
		txs, terr := chain.BlockTxs(ctx, hash, 0)
		return blockLoadedMsg{block: b, txs: txs, err: terr}
	}
}

func fetchAddress(ctx context.Context, chain provider.ChainProvider, addr string) tea.Cmd {
	return func() tea.Msg {
		a, err := chain.Address(ctx, addr)
		if err != nil {
			return addressLoadedMsg{err: err}
		}
		txs, err := chain.AddressTxs(ctx, addr, "")
		return addressLoadedMsg{address: a, txs: txs, err: err}
	}
}

// addressHistorySamplePages caps how many pages of an address's own tx
// history fetchAddressHistory walks — an active address can run into the
// thousands, and unlike the block treemap's sample this must be fetched
// sequentially (esplora paginates by last-seen-txid cursor, so page N+1
// needs page N's last txid first), so it's a real one-time serial cost per
// address view, not just a request-count.
const addressHistorySamplePages = 12 // ≈600 tx (first page up to 50, rest up to 25 each)

type addressHistoryMsg struct {
	series    []float64 // running balance in sats, chronological (oldest -> newest)
	truncated bool
	features  []string // enrich.AddressFeatures, observed over the same tx sample
}

// netEffect is a tx's net effect on addr's balance: what it paid in minus
// what it spent from addr's own previous outputs.
func netEffect(tx domain.Tx, addr string) int64 {
	var net int64
	for _, v := range tx.Vout {
		if v.Address == addr {
			net += v.Value
		}
	}
	for _, v := range tx.Vin {
		if v.Address == addr {
			net -= v.Value
		}
	}
	return net
}

// fetchAddressHistory reconstructs a balance-over-time series by walking
// the address's tx history backwards from its current confirmed balance:
// each tx's net effect on the address tells us what the balance was
// immediately before that tx. There's no "balance over time" endpoint on
// any Esplora-shaped API — this is the only honest way to chart it, and
// it's capped and flagged (truncated) rather than presented as complete
// once an address has more history than addressHistorySamplePages covers.
func fetchAddressHistory(ctx context.Context, chain provider.ChainProvider, addr string, currentBalance int64) tea.Cmd {
	return func() tea.Msg {
		var all []domain.Tx
		lastSeen := ""
		truncated := false
		for i := range addressHistorySamplePages {
			page, err := chain.AddressTxs(ctx, addr, lastSeen)
			if err != nil || len(page) == 0 {
				break
			}
			all = append(all, page...)
			lastSeen = page[len(page)-1].TxID
			if len(page) < txPageSize {
				break
			}
			if i == addressHistorySamplePages-1 {
				truncated = true
			}
		}

		// all is newest-first; balances[i] is the balance immediately
		// after all[i] (balances[0] is "now", i.e. currentBalance).
		balances := make([]float64, len(all))
		balance := currentBalance
		for i, tx := range all {
			balances[i] = float64(balance)
			balance -= netEffect(tx, addr)
		}
		series := make([]float64, len(balances))
		for i, v := range balances {
			series[len(balances)-1-i] = v
		}
		return addressHistoryMsg{series: series, truncated: truncated, features: enrich.AddressFeatures(addr, all)}
	}
}

type liveBlockLoadedMsg struct {
	txs []domain.ProjectedTx
	err error
}

func fetchProjectedBlockTxs(ctx context.Context, chain provider.ChainProvider, blockIndex int) tea.Cmd {
	return func() tea.Msg {
		txs, err := chain.ProjectedBlockTxs(ctx, blockIndex)
		return liveBlockLoadedMsg{txs: txs, err: err}
	}
}

// blockTreemapSamplePages caps how many 25-tx pages the "Actual Block"
// treemap fetches — a real block can run 100+ pages, and this is a
// one-time cost per block view (not a poll), but still bounded rather than
// unconditionally enumerating a 7,000-tx block on every open.
const blockTreemapSamplePages = 30 // ≈750 tx, well past what a terminal panel can render distinctly anyway

const blockTreemapConcurrency = 8

type blockTreemapMsg struct {
	txs       []domain.Tx
	truncated bool
}

// fetchBlockTreemap pulls up to blockTreemapSamplePages pages concurrently
// (bounded) — the block's transactions are immutable once confirmed, so
// there's no ordering or consistency concern fetching them out of order.
func fetchBlockTreemap(ctx context.Context, chain provider.ChainProvider, hash string, totalTx int) tea.Cmd {
	return func() tea.Msg {
		pages := max((totalTx+txPageSize-1)/txPageSize, 1)
		truncated := pages > blockTreemapSamplePages
		if truncated {
			pages = blockTreemapSamplePages
		}

		results := make([][]domain.Tx, pages)
		sem := make(chan struct{}, blockTreemapConcurrency)
		var wg sync.WaitGroup
		for i := range pages {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				txs, err := chain.BlockTxs(ctx, hash, i*txPageSize)
				if err == nil {
					results[i] = txs
				}
			}(i)
		}
		wg.Wait()

		var all []domain.Tx
		for _, page := range results {
			all = append(all, page...)
		}
		return blockTreemapMsg{txs: all, truncated: truncated}
	}
}

// walletGapLimit is the standard BIP44/49/84 convention: a receive or
// change chain is considered exhausted once this many consecutive
// addresses turn up with no activity at all. It's not configurable because
// it isn't hexplore's convention to pick — it's the number every
// compliant wallet already uses, and any address beyond a real gap this
// wide wouldn't be discoverable by one either.
const walletGapLimit = 20

// walletScanConcurrency bounds how many Address() lookups run at once per
// batch — same spirit as blockTreemapConcurrency.
const walletScanConcurrency = 8

// walletMaxScanPerChain caps the worst case (a wallet with activity spread
// across thousands of addresses) rather than scanning unbounded — flagged
// as Truncated, same honesty pattern as the block treemap and address
// history samples.
const walletMaxScanPerChain = 2000

type walletScanMsg struct {
	wallet domain.Wallet
	err    error
}

type walletAddrResult struct {
	index uint32
	addr  string
	stats domain.Address
	used  bool
	err   error
}

// walletScanBatch derives and looks up walletGapLimit consecutive addresses
// starting at startIndex on the given chain, bounded-concurrency, same
// pattern as fetchBlockTreemap's semaphore.
func walletScanBatch(ctx context.Context, chain provider.ChainProvider, key *wallet.Key, chainIdx, startIndex uint32) []walletAddrResult {
	results := make([]walletAddrResult, walletGapLimit)
	sem := make(chan struct{}, walletScanConcurrency)
	var wg sync.WaitGroup
	for i := range uint32(walletGapLimit) {
		wg.Add(1)
		sem <- struct{}{}
		go func(i uint32) {
			defer wg.Done()
			defer func() { <-sem }()
			idx := startIndex + i
			addr, err := key.Address(chainIdx, idx)
			if err != nil {
				results[i] = walletAddrResult{index: idx, err: err}
				return
			}
			stats, err := chain.Address(ctx, addr)
			used := err == nil && (stats.TxCount > 0 || stats.MempoolTxCount > 0)
			results[i] = walletAddrResult{index: idx, addr: addr, stats: stats, used: used, err: err}
		}(i)
	}
	wg.Wait()
	return results
}

// fetchWalletScan discovers every used address on an xpub/ypub/zpub's
// receive (chain 0) and change (chain 1) derivation chains — there's no
// chain API for this at all, since HD wallets are a client-side derivation
// convention, not a chain concept; this is the read-only side of it, an
// extended PUBLIC key never touches a private key or signs anything (see
// internal/wallet's package doc).
func fetchWalletScan(ctx context.Context, chain provider.ChainProvider, extKey string) tea.Cmd {
	return func() tea.Msg {
		key, err := wallet.Parse(extKey)
		if err != nil {
			return walletScanMsg{err: err}
		}
		return scanWallet(ctx, chain, key, extKey)
	}
}

// fetchWalletScanAs is fetchWalletScan for the ambiguous-prefix case (see
// wallet.AmbiguousScriptType): the caller has already asked the user which
// convention a plain xpub/tpub means and hands the choice in explicitly,
// overriding Parse's Legacy default.
func fetchWalletScanAs(ctx context.Context, chain provider.ChainProvider, extKey string, st wallet.ScriptType) tea.Cmd {
	return func() tea.Msg {
		key, err := wallet.Parse(extKey)
		if err != nil {
			return walletScanMsg{err: err}
		}
		key.ScriptType = st
		return scanWallet(ctx, chain, key, extKey)
	}
}

func scanWallet(ctx context.Context, chain provider.ChainProvider, key *wallet.Key, extKey string) walletScanMsg {
	var addrs []domain.WalletAddress
	truncated := false
	for chainIdx := range uint32(2) {
		consecutiveEmpty := 0
		for nextIndex := uint32(0); consecutiveEmpty < walletGapLimit; nextIndex += walletGapLimit {
			if nextIndex >= walletMaxScanPerChain {
				truncated = true
				break
			}
			for _, r := range walletScanBatch(ctx, chain, key, chainIdx, nextIndex) {
				if r.err != nil {
					continue // a provider hiccup on one address shouldn't derail the whole scan
				}
				if r.used {
					consecutiveEmpty = 0
					addrs = append(addrs, domain.WalletAddress{Address: r.addr, Chain: chainIdx, Index: r.index, Stats: r.stats})
				} else {
					consecutiveEmpty++
				}
			}
		}
	}

	return walletScanMsg{wallet: domain.Wallet{
		Key: extKey, ScriptType: key.ScriptType.String(),
		Addresses: addrs, GapLimit: walletGapLimit, Truncated: truncated,
	}}
}

// watchlistFetchConcurrency bounds how many watched items (addresses and
// wallets alike) are looked up at once — a wallet's own scan is already
// internally bounded (walletScanConcurrency), so this only limits how many
// top-level watchlist entries are in flight together.
const watchlistFetchConcurrency = 6

type watchlistStatsMsg struct {
	addrStats   map[string]domain.Address
	walletStats map[string]domain.Wallet
}

// fetchWatchlistStats enriches the watchlist with live data for its tile
// view: one Address() lookup per watched address, and a full gap-limit
// scan per watched wallet (reusing scanWallet — the exact same aggregate a
// direct wallet open would show). scriptTypeOverrides is a snapshot (not a
// live reference) of resolved xpub/tpub choices, since this runs in its
// own goroutines while the main model can keep mutating its own map.
func fetchWatchlistStats(
	ctx context.Context, chain provider.ChainProvider,
	addrs []cache.WatchedAddress, wallets []cache.WatchedWallet,
	scriptTypeOverrides map[string]wallet.ScriptType,
) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		addrStats := make(map[string]domain.Address, len(addrs))
		walletStats := make(map[string]domain.Wallet, len(wallets))

		sem := make(chan struct{}, watchlistFetchConcurrency)
		var wg sync.WaitGroup

		for _, a := range addrs {
			wg.Add(1)
			sem <- struct{}{}
			go func(addr string) {
				defer wg.Done()
				defer func() { <-sem }()
				stats, err := chain.Address(ctx, addr)
				if err != nil {
					return
				}
				mu.Lock()
				addrStats[addr] = stats
				mu.Unlock()
			}(a.Address)
		}

		for _, w := range wallets {
			wg.Add(1)
			sem <- struct{}{}
			go func(ww cache.WatchedWallet) {
				defer wg.Done()
				defer func() { <-sem }()
				key, err := wallet.Parse(ww.Key)
				if err != nil {
					return
				}
				if st, ok := wallet.ParseScriptTypeName(ww.ScriptType); ok {
					key.ScriptType = st
				} else if st, ok := scriptTypeOverrides[ww.Key]; ok {
					key.ScriptType = st
				}
				msg := scanWallet(ctx, chain, key, ww.Key)
				mu.Lock()
				walletStats[ww.Key] = msg.wallet
				mu.Unlock()
			}(w)
		}

		wg.Wait()
		return watchlistStatsMsg{addrStats: addrStats, walletStats: walletStats}
	}
}
