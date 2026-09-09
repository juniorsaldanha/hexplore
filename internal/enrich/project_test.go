package enrich

import (
	"testing"

	"github.com/juniorsaldanha/hexplore/internal/domain"
)

func TestProjectNextBlocksSingleBlock(t *testing.T) {
	m := domain.MempoolState{
		Histogram: [][2]float64{
			{10, 400_000},
			{5, 400_000},
			{1, 400_000},
		},
	}
	blocks := ProjectNextBlocks(m, 3)
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (800k left after first fills)", len(blocks))
	}

	b0 := blocks[0]
	if b0.VSize != blockVSize {
		t.Errorf("block0 VSize = %d, want %d", b0.VSize, int64(blockVSize))
	}
	// the crossing bucket (feerate 1) is only partially consumed, so it still
	// sets the floor: that's the min fee to make it into this block.
	if b0.MaxFeeRate != 10 || b0.MinFeeRate != 1 {
		t.Errorf("block0 rate range = [%v,%v], want [1,10]", b0.MinFeeRate, b0.MaxFeeRate)
	}
	// consumed 400k@10 + 400k@5 + 200k@1 = 1_000_000 vB total fee:
	wantFee := 400_000*10 + 400_000*5 + 200_000*1
	if int64(wantFee) != b0.TotalFeeSats {
		t.Errorf("block0 TotalFeeSats = %d, want %d", b0.TotalFeeSats, wantFee)
	}
	if !b0.Approx {
		t.Error("block0 Approx = false, want true")
	}

	b1 := blocks[1]
	if b1.VSize != 200_000 {
		t.Errorf("block1 VSize = %d, want 200000 (the 1@1 remainder)", b1.VSize)
	}
}

func TestProjectNextBlocksEmpty(t *testing.T) {
	if got := ProjectNextBlocks(domain.MempoolState{}, 3); len(got) != 0 {
		t.Errorf("got %d blocks from empty histogram, want 0", len(got))
	}
}

func TestWeightedMedianRate(t *testing.T) {
	buckets := [][2]float64{{10, 100}, {5, 100}, {1, 100}}
	if got := weightedMedianRate(buckets); got != 5 {
		t.Errorf("weightedMedianRate = %v, want 5", got)
	}
}
