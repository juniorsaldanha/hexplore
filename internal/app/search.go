package app

import "regexp"

// dispatchKind and the regexes below implement docs/PLAN.md §7 — shape-based
// dispatch, no guessing round-trips. The 64-hex ambiguity (block hash vs
// txid) is resolved by trying tx first and falling back to block on error,
// per the plan; the leading-zero-vs-difficulty heuristic is a nice-to-have
// left for later.
type dispatchKind int

const (
	dispatchNone dispatchKind = iota
	dispatchHeight
	dispatchHash // ambiguous 64-hex: try tx, fall back to block
	dispatchAddress
	dispatchWallet // xpub/ypub/zpub and testnet counterparts — see internal/wallet
)

var (
	reHeight      = regexp.MustCompile(`^[0-9]{1,7}$`)
	reHex64       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reBase58      = regexp.MustCompile(`^(1|3)[a-km-zA-HJ-NP-Z1-9]{25,34}$`)
	reBech32      = regexp.MustCompile(`^bc1[qp][a-z0-9]{38,58}$`)
	reExtendedKey = regexp.MustCompile(`^[xyztuv](pub|prv)[1-9A-HJ-NP-Za-km-z]{107}$`)
)

func classify(input string) dispatchKind {
	switch {
	case reHeight.MatchString(input):
		return dispatchHeight
	case reHex64.MatchString(input):
		return dispatchHash
	case reBase58.MatchString(input), reBech32.MatchString(input):
		return dispatchAddress
	case reExtendedKey.MatchString(input):
		return dispatchWallet
	default:
		return dispatchNone
	}
}
