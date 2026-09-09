package esplora

import (
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

// JSON shapes match blockstream.info/mempool.space's Esplora REST surface
// (verified against https://mempool.space/api/... — see docs/PLAN.md §13.2).

type blockJSON struct {
	ID                string  `json:"id"`
	Height            int     `json:"height"`
	Version           int32   `json:"version"`
	Timestamp         int64   `json:"timestamp"`
	Bits              uint32  `json:"bits"`
	Nonce             uint32  `json:"nonce"`
	Difficulty        float64 `json:"difficulty"`
	MerkleRoot        string  `json:"merkle_root"`
	TxCount           int     `json:"tx_count"`
	Size              int64   `json:"size"`
	Weight            int64   `json:"weight"`
	PreviousBlockHash string  `json:"previousblockhash"`
}

func (b blockJSON) toDomain() domain.Block {
	return domain.Block{
		Height:     b.Height,
		Hash:       b.ID,
		PrevHash:   b.PreviousBlockHash,
		Time:       time.Unix(b.Timestamp, 0),
		TxCount:    b.TxCount,
		Size:       b.Size,
		Weight:     b.Weight,
		Version:    b.Version,
		Bits:       b.Bits,
		Nonce:      b.Nonce,
		MerkleRoot: b.MerkleRoot,
		Difficulty: b.Difficulty,
	}
}

type prevoutJSON struct {
	ScriptPubKeyAddress string `json:"scriptpubkey_address"`
	Value               int64  `json:"value"`
}

type vinJSON struct {
	TxID       string       `json:"txid"`
	Vout       int          `json:"vout"`
	Prevout    *prevoutJSON `json:"prevout"`
	IsCoinbase bool         `json:"is_coinbase"`
}

type voutJSON struct {
	ScriptPubKeyAddress string `json:"scriptpubkey_address"`
	Value               int64  `json:"value"`
}

type statusJSON struct {
	Confirmed   bool   `json:"confirmed"`
	BlockHeight int    `json:"block_height"`
	BlockHash   string `json:"block_hash"`
	BlockTime   int64  `json:"block_time"`
}

type txJSON struct {
	TxID     string     `json:"txid"`
	Version  int        `json:"version"`
	Locktime uint32     `json:"locktime"`
	Size     int64      `json:"size"`
	Weight   int64      `json:"weight"`
	Fee      int64      `json:"fee"`
	Vin      []vinJSON  `json:"vin"`
	Vout     []voutJSON `json:"vout"`
	Status   statusJSON `json:"status"`
}

func (t txJSON) toDomain() domain.Tx {
	vin := make([]domain.Vin, len(t.Vin))
	for i, v := range t.Vin {
		dv := domain.Vin{TxID: v.TxID, Vout: v.Vout, Coinbase: v.IsCoinbase}
		if v.Prevout != nil {
			dv.Address = v.Prevout.ScriptPubKeyAddress
			dv.Value = v.Prevout.Value
		}
		vin[i] = dv
	}
	vout := make([]domain.Vout, len(t.Vout))
	for i, v := range t.Vout {
		vout[i] = domain.Vout{Address: v.ScriptPubKeyAddress, Value: v.Value}
	}
	return domain.Tx{
		TxID:     t.TxID,
		Version:  t.Version,
		Locktime: t.Locktime,
		Size:     t.Size,
		Weight:   t.Weight,
		Fee:      t.Fee,
		Vin:      vin,
		Vout:     vout,
		Status: domain.TxStatus{
			Confirmed:   t.Status.Confirmed,
			BlockHeight: t.Status.BlockHeight,
			BlockHash:   t.Status.BlockHash,
			BlockTime:   time.Unix(t.Status.BlockTime, 0),
		},
	}
}

type addressStatsJSON struct {
	FundedTxoCount int   `json:"funded_txo_count"`
	FundedTxoSum   int64 `json:"funded_txo_sum"`
	SpentTxoCount  int   `json:"spent_txo_count"`
	SpentTxoSum    int64 `json:"spent_txo_sum"`
	TxCount        int   `json:"tx_count"`
}

type addressJSON struct {
	Address      string           `json:"address"`
	ChainStats   addressStatsJSON `json:"chain_stats"`
	MempoolStats addressStatsJSON `json:"mempool_stats"`
}

func (a addressJSON) toDomain() domain.Address {
	return domain.Address{
		Address:               a.Address,
		FundedSats:            a.ChainStats.FundedTxoSum,
		SpentSats:             a.ChainStats.SpentTxoSum,
		TxCount:               a.ChainStats.TxCount,
		FundedTxoCount:        a.ChainStats.FundedTxoCount,
		SpentTxoCount:         a.ChainStats.SpentTxoCount,
		MempoolFundedSats:     a.MempoolStats.FundedTxoSum,
		MempoolSpentSats:      a.MempoolStats.SpentTxoSum,
		MempoolTxCount:        a.MempoolStats.TxCount,
		MempoolFundedTxoCount: a.MempoolStats.FundedTxoCount,
	}
}

type mempoolJSON struct {
	Count        int          `json:"count"`
	VSize        int64        `json:"vsize"`
	TotalFee     int64        `json:"total_fee"`
	FeeHistogram [][2]float64 `json:"fee_histogram"`
}

func (m mempoolJSON) toDomain() domain.MempoolState {
	return domain.MempoolState{
		Count:        m.Count,
		VSize:        m.VSize,
		TotalFeeSats: m.TotalFee,
		Histogram:    m.FeeHistogram,
	}
}
