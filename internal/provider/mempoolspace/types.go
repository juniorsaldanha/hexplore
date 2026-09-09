package mempoolspace

import (
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

// JSON shapes match mempool.space's /v1 extras, verified against
// https://mempool.space/api/v1/... — see docs/PLAN.md §13.2.

type v1PoolJSON struct {
	Name string `json:"name"`
}

type v1ExtrasJSON struct {
	TotalFees      int64      `json:"totalFees"`
	AvgFee         int64      `json:"avgFee"`
	Pool           v1PoolJSON `json:"pool"`
	FeeRange       []float64  `json:"feeRange"`     // percentiles ascending; [0]=min, [last]=max
	MatchRate      float64    `json:"matchRate"`    // 0-100, how closely the mined block matched the predicted template
	ExpectedFees   int64      `json:"expectedFees"` // what the mempool predicted just before this block was mined
	ExpectedWeight int64      `json:"expectedWeight"`
}

type v1BlockJSON struct {
	ID                string       `json:"id"`
	Height            int          `json:"height"`
	Version           int32        `json:"version"`
	Timestamp         int64        `json:"timestamp"`
	Bits              uint32       `json:"bits"`
	Nonce             uint32       `json:"nonce"`
	Difficulty        float64      `json:"difficulty"`
	MerkleRoot        string       `json:"merkle_root"`
	TxCount           int          `json:"tx_count"`
	Size              int64        `json:"size"`
	Weight            int64        `json:"weight"`
	PreviousBlockHash string       `json:"previousblockhash"`
	Extras            v1ExtrasJSON `json:"extras"`
}

func (b v1BlockJSON) toDomain() domain.Block {
	var avgFeeRate float64
	if vsize := float64(b.Weight) / 4; vsize > 0 {
		avgFeeRate = float64(b.Extras.TotalFees) / vsize
	}
	fee := &domain.BlockFee{
		TotalSats:  b.Extras.TotalFees,
		AvgSats:    b.Extras.AvgFee,
		AvgFeeRate: avgFeeRate,
		Approx:     false,
	}
	if n := len(b.Extras.FeeRange); n > 0 {
		fee.MinFeeRate = b.Extras.FeeRange[0]
		fee.MaxFeeRate = b.Extras.FeeRange[n-1]
	}
	return domain.Block{
		Height:          b.Height,
		Hash:            b.ID,
		PrevHash:        b.PreviousBlockHash,
		Time:            time.Unix(b.Timestamp, 0),
		TxCount:         b.TxCount,
		Size:            b.Size,
		Weight:          b.Weight,
		Version:         b.Version,
		Bits:            b.Bits,
		Nonce:           b.Nonce,
		MerkleRoot:      b.MerkleRoot,
		Difficulty:      b.Difficulty,
		Fee:             fee,
		Pool:            b.Extras.Pool.Name,
		MatchRate:       b.Extras.MatchRate,
		ExpectedFeeSats: b.Extras.ExpectedFees,
		ExpectedWeight:  b.Extras.ExpectedWeight,
	}
}

type v1FeesJSON struct {
	FastestFee  float64 `json:"fastestFee"`
	HalfHourFee float64 `json:"halfHourFee"`
	HourFee     float64 `json:"hourFee"`
	EconomyFee  float64 `json:"economyFee"`
}

func (f v1FeesJSON) toDomain() domain.FeeTiers {
	return domain.FeeTiers{
		HighSatVB:    f.FastestFee,
		AvgSatVB:     f.HalfHourFee,
		LowSatVB:     f.HourFee,
		EconomySatVB: f.EconomyFee,
	}
}

type v1MempoolBlockJSON struct {
	BlockVSize float64   `json:"blockVSize"`
	NTx        int       `json:"nTx"`
	TotalFees  int64     `json:"totalFees"`
	MedianFee  float64   `json:"medianFee"`
	FeeRange   []float64 `json:"feeRange"`
}

func (b v1MempoolBlockJSON) toDomain() domain.ProjectedBlock {
	pb := domain.ProjectedBlock{
		VSize:         int64(b.BlockVSize),
		NTx:           b.NTx,
		TotalFeeSats:  b.TotalFees,
		MedianFeeRate: b.MedianFee,
		Approx:        false,
	}
	if len(b.FeeRange) > 0 {
		pb.MinFeeRate = b.FeeRange[0]
		pb.MaxFeeRate = b.FeeRange[len(b.FeeRange)-1]
	}
	return pb
}
