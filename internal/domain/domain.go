// Package domain holds provider-neutral types. Nothing here imports a provider package.
package domain

import "time"

// Capabilities declares what a provider can do natively. Callers check these
// before relying on a value that some providers can only approximate or not
// produce at all — see ErrUnsupported in package provider.
type Capabilities struct {
	BlockFees      bool // totalFees/avgFee without derivation
	MiningPool     bool
	NextBlockExact bool // real projection vs histogram approximation
	AddressPrefix  bool // autocomplete search
	WebSocket      bool // push updates vs polling
	LiveBlockTxs   bool // per-tx projected-next-block feed (mempool.space's /v1/ws only)
	Testnet        bool
	Signet         bool
}

// Block is a confirmed block. Fee and Pool are derived/native and left at
// their zero value when the active provider can't supply them.
type Block struct {
	Height   int
	Hash     string
	PrevHash string
	Time     time.Time
	TxCount  int
	Size     int64 // bytes
	Weight   int64 // weight units; fill % = Weight / 4_000_000

	Version    int32
	Bits       uint32
	Nonce      uint32
	MerkleRoot string
	Difficulty float64

	Fee  *BlockFee // nil until derived/fetched
	Pool string    // mining pool name, empty if unknown or unsupported

	// MatchRate is mempool.space's own "how closely did the mined block
	// match what was predicted" percentage — 0 when unavailable (only
	// native mempool.space block data carries it, never derived).
	MatchRate float64

	// ExpectedFeeSats/ExpectedWeight are what the mempool predicted for
	// this block just before it was mined — aggregate numbers only, native
	// mempool.space data, both 0 when unavailable. There's no equivalent
	// per-transaction breakdown available for a historical block anywhere
	// in the public API (only mempool.space's own internal audit storage
	// has that), so there's no way to render an "expected" treemap to sit
	// alongside the real one — only this numeric comparison.
	ExpectedFeeSats int64
	ExpectedWeight  int64
}

// BlockFee is the total and average fee for a block, either returned
// natively by the provider or derived from the coinbase output.
type BlockFee struct {
	TotalSats  int64
	AvgSats    int64
	AvgFeeRate float64 // sat/vB
	Approx     bool    // true when derived rather than provider-native
	Burned     bool    // coinbase claimed less than full subsidy (blocks 124724, 501726)

	// MinFeeRate/MaxFeeRate are the block's fee-rate span (sat/vB) — both 0
	// when unavailable (native mempool.space data only, from its feeRange
	// percentiles; never derived).
	MinFeeRate float64
	MaxFeeRate float64
}

// ProjectedBlock is a not-yet-mined block, either the provider's own
// projection or one derived from the mempool fee histogram (see
// docs/PLAN.md §5). Zero NTx/TotalFeeSats mean the source couldn't supply
// them — the histogram gives vsize and feerate, not transaction counts.
type ProjectedBlock struct {
	VSize         int64
	NTx           int
	TotalFeeSats  int64
	MedianFeeRate float64
	MinFeeRate    float64
	MaxFeeRate    float64
	Approx        bool
}

type TxStatus struct {
	Confirmed   bool
	BlockHeight int
	BlockHash   string
	BlockTime   time.Time
}

type Vin struct {
	TxID     string
	Vout     int
	Address  string
	Value    int64
	Coinbase bool
}

type Vout struct {
	Address string
	Value   int64
}

type Tx struct {
	TxID     string
	Version  int
	Locktime uint32
	Size     int64
	Weight   int64
	Fee      int64 // sats
	RBF      bool
	Vin      []Vin
	Vout     []Vout
	Status   TxStatus
}

// Address is chain-stats only; mempool deltas are tracked separately since
// they change on a different cadence.
type Address struct {
	Address           string
	FundedSats        int64
	SpentSats         int64
	TxCount           int
	MempoolFundedSats int64
	MempoolSpentSats  int64
	MempoolTxCount    int

	// FundedTxoCount/SpentTxoCount are confirmed output counters straight
	// from the provider — how many outputs have ever paid this address,
	// and how many of those are already spent. ConfirmedUTXOs derives the
	// live unspent count from them.
	FundedTxoCount int
	SpentTxoCount  int
	// MempoolFundedTxoCount is how many new, not-yet-confirmed outputs are
	// incoming to this address right now.
	MempoolFundedTxoCount int
}

func (a Address) BalanceSats() int64 {
	return a.FundedSats - a.SpentSats
}

// ConfirmedUTXOs is the address's live unspent-output count.
func (a Address) ConfirmedUTXOs() int {
	return a.FundedTxoCount - a.SpentTxoCount
}

// FeeTiers maps confirmation targets to a sat/vB estimate. EconomySatVB is 0
// when the provider doesn't offer a fourth tier.
type FeeTiers struct {
	HighSatVB    float64
	AvgSatVB     float64
	LowSatVB     float64
	EconomySatVB float64
}

// MempoolState is the current unconfirmed pool. Histogram is
// [feerate, vsize] pairs descending by feerate, used to project the next
// block (see docs/PLAN.md §4).
type MempoolState struct {
	Count        int
	VSize        int64
	TotalFeeSats int64
	Histogram    [][2]float64
}

type Price struct {
	Currency  string
	Value     float64
	Change24h float64 // percent
}

// WalletAddress is one address discovered while scanning an extended
// public key (see internal/wallet) that turned out to have real activity —
// Chain 0 is the receive/external derivation chain, 1 is change/internal,
// per BIP44/49/84.
type WalletAddress struct {
	Address string
	Chain   uint32
	Index   uint32
	Stats   Address
}

// Wallet is the result of scanning an xpub/ypub/zpub for used addresses via
// the standard gap-limit algorithm — an aggregate built client-side from
// many individual Address lookups, since no chain API knows about HD
// wallets at all. ScriptType is a human label (internal/wallet.ScriptType's
// String()), kept as plain text here since domain stays free of every
// other internal package too.
type Wallet struct {
	Key        string
	ScriptType string
	Addresses  []WalletAddress
	GapLimit   int  // consecutive unused addresses per chain that stopped the scan
	Truncated  bool // hit the scan's own hard cap before either chain's gap limit
}

func (w Wallet) BalanceSats() int64 {
	var total int64
	for _, a := range w.Addresses {
		total += a.Stats.BalanceSats()
	}
	return total
}

func (w Wallet) PendingBalanceSats() int64 {
	var total int64
	for _, a := range w.Addresses {
		total += a.Stats.MempoolFundedSats - a.Stats.MempoolSpentSats
	}
	return total
}

func (w Wallet) ReceivedSats() int64 {
	var total int64
	for _, a := range w.Addresses {
		total += a.Stats.FundedSats
	}
	return total
}

func (w Wallet) ConfirmedUTXOs() int {
	var total int
	for _, a := range w.Addresses {
		total += a.Stats.ConfirmedUTXOs()
	}
	return total
}

func (w Wallet) PendingUTXOs() int {
	var total int
	for _, a := range w.Addresses {
		total += a.Stats.MempoolFundedTxoCount
	}
	return total
}

func (w Wallet) TxCount() int {
	var total int
	for _, a := range w.Addresses {
		total += a.Stats.TxCount
	}
	return total
}

// ProjectedTx is one transaction in the live projected-next-block feed —
// mempool.space's /v1/ws "projected-block-transactions" message. Flags is
// their raw classification bitmask (see enrich.ClassifyFlags); it's carried
// here unset (0) rather than guessed for providers that can't supply it.
type ProjectedTx struct {
	TxID      string
	FeeSats   int64
	VSize     float64 // vbytes; fractional because it's weight/4
	ValueSats int64
	FeeRate   float64 // sat/vB
	Flags     uint64
	FirstSeen time.Time
}
