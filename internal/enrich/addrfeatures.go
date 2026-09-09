package enrich

import "github.com/juniorsaldanha/hexplore/internal/domain"

// AddressFeatures lists the notable technical characteristics an address
// actually shows — not a single exclusive "type" (an address is always
// exactly one script type, see ClassifyAddress), but a set of independent
// traits that can co-occur: a Taproot address is also SegWit; an address
// can both signal RBF on some transactions and participate in a CoinJoin
// on others.
//
// txs is a sample of the address's own transactions (as fetched by
// app.fetchAddressHistory) — RBF/CoinJoin/consolidation/coinbase are
// observed from that sample, not decreed about the address as a whole, so
// this can miss something true of the address's full history when the
// sample is truncated.
func AddressFeatures(addr string, txs []domain.Tx) []string {
	var feats []string
	switch ClassifyAddress(addr) {
	case "P2TR (Taproot)":
		feats = append(feats, "SegWit", "Taproot")
	case "P2WPKH", "P2WSH":
		feats = append(feats, "SegWit")
	}

	var sawRBF, sawCoinJoin, sawConsolidation, sawCoinbase bool
	for _, tx := range txs {
		if tx.RBF {
			sawRBF = true
		}
		switch ClassifyTx(tx) {
		case TxCoinJoin:
			sawCoinJoin = true
		case TxConsolidation:
			sawConsolidation = true
		case TxCoinbase:
			sawCoinbase = true
		}
	}
	if sawRBF {
		feats = append(feats, "RBF")
	}
	if sawCoinJoin {
		feats = append(feats, "CoinJoin")
	}
	if sawConsolidation {
		feats = append(feats, "Consolidation")
	}
	if sawCoinbase {
		feats = append(feats, "Coinbase Payouts")
	}
	return feats
}
