// Package enrich derives values no chain API returns directly: block fee
// totals from the coinbase output, next-block projections from the mempool
// fee histogram, and the sat/vB → fiat conversion. Every function here is
// pure — providers fetch, enrich computes.
package enrich

import "github.com/juniorsaldanha/hexplore/internal/domain"

// Subsidy returns the block subsidy in sats at height, per Bitcoin's halving
// schedule (50 BTC, halving every 210,000 blocks, zero after 33 halvings).
func Subsidy(height int) int64 {
	halvings := height / 210_000
	if halvings >= 34 {
		return 0
	}
	return 5_000_000_000 >> uint(halvings)
}

// CoinbaseFees derives the total fees collected in a block from the sum of
// its coinbase output values. A handful of blocks (124724, 501726) provably
// burned part of the subsidy, which would otherwise show as a negative fee
// total — those are clamped to zero and flagged.
func CoinbaseFees(coinbaseVoutSum int64, height int) (totalSats int64, burned bool) {
	total := coinbaseVoutSum - Subsidy(height)
	if total < 0 {
		return 0, true
	}
	return total, false
}

// BlockFee turns a coinbase output sum into the full domain.BlockFee,
// dividing across the block's weight and transaction count. Weight is in
// weight units (so /4 for vbytes); txCount includes the coinbase itself.
func BlockFee(coinbaseVoutSum int64, height int, weight int64, txCount int) domain.BlockFee {
	total, burned := CoinbaseFees(coinbaseVoutSum, height)

	var avgFeeRate float64
	if vsize := float64(weight) / 4; vsize > 0 {
		avgFeeRate = float64(total) / vsize
	}
	var avgSats int64
	if txCount > 1 {
		avgSats = total / int64(txCount-1)
	}
	return domain.BlockFee{
		TotalSats:  total,
		AvgSats:    avgSats,
		AvgFeeRate: avgFeeRate,
		Approx:     true,
		Burned:     burned,
	}
}
