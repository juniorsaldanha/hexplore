package enrich

import (
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

func vins(n int) []domain.Vin {
	out := make([]domain.Vin, n)
	return out
}

func voutsOfValue(value int64, n int) []domain.Vout {
	out := make([]domain.Vout, n)
	for i := range out {
		out[i] = domain.Vout{Value: value}
	}
	return out
}

func TestClassifyTxCoinbase(t *testing.T) {
	tx := domain.Tx{Vin: []domain.Vin{{Coinbase: true}}, Vout: voutsOfValue(5_000_000_000, 1)}
	if got := ClassifyTx(tx); got != TxCoinbase {
		t.Errorf("ClassifyTx(coinbase) = %v, want TxCoinbase", got)
	}
}

func TestClassifyTxConsolidation(t *testing.T) {
	tx := domain.Tx{Vin: vins(5), Vout: voutsOfValue(100_000, 1)}
	if got := ClassifyTx(tx); got != TxConsolidation {
		t.Errorf("ClassifyTx(5-in-1-out) = %v, want TxConsolidation", got)
	}
	// exactly at the input threshold
	tx3 := domain.Tx{Vin: vins(3), Vout: voutsOfValue(100_000, 2)}
	if got := ClassifyTx(tx3); got != TxConsolidation {
		t.Errorf("ClassifyTx(3-in-2-out) = %v, want TxConsolidation", got)
	}
}

func TestClassifyTxCoinJoin(t *testing.T) {
	vout := append(voutsOfValue(100_000, 6), domain.Vout{Value: 234_567}) // 6 equal + 1 change
	tx := domain.Tx{Vin: vins(6), Vout: vout}
	if got := ClassifyTx(tx); got != TxCoinJoin {
		t.Errorf("ClassifyTx(6 equal outputs) = %v, want TxCoinJoin", got)
	}
}

func TestClassifyTxNormal(t *testing.T) {
	tx := domain.Tx{Vin: vins(2), Vout: voutsOfValue(50_000, 2)}
	if got := ClassifyTx(tx); got != TxNormal {
		t.Errorf("ClassifyTx(2-in-2-out) = %v, want TxNormal", got)
	}
}

func TestClassifyTxIgnoresZeroValueOutputsForCoinJoin(t *testing.T) {
	// 6 zero-value (OP_RETURN-style) outputs must not look like a CoinJoin round.
	vout := append(voutsOfValue(0, 6), domain.Vout{Value: 40_000})
	tx := domain.Tx{Vin: vins(2), Vout: vout}
	if got := ClassifyTx(tx); got != TxNormal {
		t.Errorf("ClassifyTx(zero-value outputs) = %v, want TxNormal, not a false CoinJoin", got)
	}
}

func TestClassifyFlags(t *testing.T) {
	cases := []struct {
		name  string
		flags uint64
		want  TxKind
	}{
		{"plain segwit v2 rbf tx (real sample)", 1099511631881, TxNormal},
		{"op_return batch (real sample)", 16777226, TxData},
		{"coinjoin bit set", flagCoinJoin, TxCoinJoin},
		{"consolidation bit set", flagConsolidation, TxConsolidation},
		{"inscription bit set", flagInscription, TxData},
		{"coinjoin wins over consolidation if both set", flagCoinJoin | flagConsolidation, TxCoinJoin},
		{"no relevant bits", 0, TxNormal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyFlags(c.flags); got != c.want {
				t.Errorf("ClassifyFlags(%d) = %v, want %v", c.flags, got, c.want)
			}
		})
	}
}

func TestClassifyTxFewInputsIsNotConsolidation(t *testing.T) {
	tx := domain.Tx{Vin: vins(2), Vout: voutsOfValue(50_000, 1)}
	if got := ClassifyTx(tx); got != TxNormal {
		t.Errorf("ClassifyTx(2-in-1-out) = %v, want TxNormal (below the input threshold)", got)
	}
}
