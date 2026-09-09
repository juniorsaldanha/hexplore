package enrich

import "github.com/juniorsaldanha/hexplore/internal/domain"

// TxKind is a heuristic classification, not a proof — there's no wallet-
// fingerprint database behind this, just the input/output shape.
type TxKind int

const (
	TxNormal TxKind = iota
	TxCoinbase
	TxConsolidation // many inputs swept into one or two outputs
	TxCoinJoin      // several outputs sharing the same value, the standard round-signature
	TxData          // carries OP_RETURN / inscription data — only derivable from real flags, see ClassifyFlags
)

func (k TxKind) String() string {
	switch k {
	case TxCoinbase:
		return "coinbase"
	case TxConsolidation:
		return "consolidation"
	case TxCoinJoin:
		return "coinjoin"
	case TxData:
		return "data"
	default:
		return "normal"
	}
}

const (
	consolidationMinInputs  = 3
	consolidationMaxOutputs = 2
	coinJoinMinEqualOutputs = 5
)

// ClassifyTx applies two simple, well-known heuristics:
//   - consolidation: >=3 inputs swept into <=2 outputs
//   - coinjoin: >=5 outputs sharing the exact same value (the standard
//     Wasabi/Whirlpool-style equal-output round signature)
//
// A coinbase input always wins, since a tx can't be both.
func ClassifyTx(tx domain.Tx) TxKind {
	for _, v := range tx.Vin {
		if v.Coinbase {
			return TxCoinbase
		}
	}
	if len(tx.Vin) >= consolidationMinInputs && len(tx.Vout) <= consolidationMaxOutputs {
		return TxConsolidation
	}
	if hasEqualOutputRound(tx.Vout) {
		return TxCoinJoin
	}
	return TxNormal
}

// Real classification bits, straight from mempool.space's own backend
// (mempool/mempool: frontend/src/app/shared/filters.utils.ts,
// TransactionFlags) — not a heuristic. /v1/ws tags every projected
// transaction with these; confirmed against the live feed.
const (
	flagCoinJoin      uint64 = 1 << 32
	flagConsolidation uint64 = 1 << 33
	flagOpReturn      uint64 = 1 << 24
	flagInscription   uint64 = 1 << 26
)

// ClassifyFlags decodes a domain.ProjectedTx.Flags value from the live
// projected-block feed. Precise, unlike ClassifyTx — the server has
// already inspected the whole transaction, not just vin/vout counts.
func ClassifyFlags(flags uint64) TxKind {
	switch {
	case flags&flagCoinJoin != 0:
		return TxCoinJoin
	case flags&flagConsolidation != 0:
		return TxConsolidation
	case flags&(flagOpReturn|flagInscription) != 0:
		return TxData
	default:
		return TxNormal
	}
}

func hasEqualOutputRound(vout []domain.Vout) bool {
	if len(vout) < coinJoinMinEqualOutputs {
		return false
	}
	counts := make(map[int64]int, len(vout))
	for _, v := range vout {
		if v.Value <= 0 {
			continue // OP_RETURN/data outputs are 0-value and would false-positive here
		}
		counts[v.Value]++
		if counts[v.Value] >= coinJoinMinEqualOutputs {
			return true
		}
	}
	return false
}
