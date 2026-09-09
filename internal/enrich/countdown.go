package enrich

import "time"

const (
	retargetInterval = 2016
	halvingInterval  = 210_000
	avgBlockTime     = 10 * time.Minute // long-run design target; the only estimate available without external hashrate data
)

// BlocksUntilRetarget is how many blocks remain until bitcoind next
// recalculates difficulty.
func BlocksUntilRetarget(height int) int {
	return retargetInterval - height%retargetInterval
}

// BlocksUntilHalving is how many blocks remain until the subsidy next halves.
func BlocksUntilHalving(height int) int {
	return halvingInterval - height%halvingInterval
}

// EstimatedETA projects a block count forward at Bitcoin's 10-minute design
// target. Always an approximation — actual block times vary with hashrate.
func EstimatedETA(blocksRemaining int) time.Duration {
	return time.Duration(blocksRemaining) * avgBlockTime
}
