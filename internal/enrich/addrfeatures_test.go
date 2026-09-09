package enrich

import (
	"reflect"
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

func TestAddressFeatures(t *testing.T) {
	const (
		taproot = "bc1p5d7rjq7g6rdk2yhzks9smlaqtedr4dekq08ge8ztwac72sfr9rusxg3297"
		segwit  = "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
		legacy  = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	)

	t.Run("taproot address with no tx history is SegWit and Taproot", func(t *testing.T) {
		got := AddressFeatures(taproot, nil)
		want := []string{"SegWit", "Taproot"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("AddressFeatures() = %v, want %v", got, want)
		}
	})

	t.Run("legacy address with no tx history has no features", func(t *testing.T) {
		if got := AddressFeatures(legacy, nil); len(got) != 0 {
			t.Errorf("AddressFeatures() = %v, want none", got)
		}
	})

	t.Run("RBF observed in the tx sample is additive to the encoding features", func(t *testing.T) {
		txs := []domain.Tx{{RBF: true}}
		got := AddressFeatures(segwit, txs)
		want := []string{"SegWit", "RBF"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("AddressFeatures() = %v, want %v", got, want)
		}
	})

	t.Run("coinjoin, consolidation and coinbase are each detected once regardless of repeat occurrences", func(t *testing.T) {
		coinjoinTx := domain.Tx{Vout: []domain.Vout{
			{Value: 100}, {Value: 100}, {Value: 100}, {Value: 100}, {Value: 100},
		}}
		consolidationTx := domain.Tx{
			Vin:  []domain.Vin{{}, {}, {}},
			Vout: []domain.Vout{{}},
		}
		coinbaseTx := domain.Tx{Vin: []domain.Vin{{Coinbase: true}}}
		txs := []domain.Tx{coinjoinTx, coinjoinTx, consolidationTx, coinbaseTx}

		got := AddressFeatures(legacy, txs)
		want := []string{"CoinJoin", "Consolidation", "Coinbase Payouts"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("AddressFeatures() = %v, want %v", got, want)
		}
	})
}
