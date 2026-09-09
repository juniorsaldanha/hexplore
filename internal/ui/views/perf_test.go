package views

import (
	"testing"
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/ui/components"
	"github.com/juniorsaldanha/hexplore/internal/ui/theme"
)

// frameBudget is the time a single render gets to stay under to sustain
// 60fps (16.67ms), with generous headroom: this only measures Dashboard's
// own string-building cost, not the terminal I/O that follows it in a real
// program, so it should complete in a small fraction of the full budget.
const frameBudget = 5 * time.Millisecond

func representativeDashboard() DashboardData {
	now := time.Now()
	blocks := make([]domain.Block, 10)
	for i := range blocks {
		blocks[i] = domain.Block{
			Height:  912304 - i,
			Hash:    "00000000000000000000000000000000000000000000000000000000000000",
			Time:    now.Add(-time.Duration(i*10) * time.Minute),
			TxCount: 3000 + i*37,
			Size:    1_500_000,
			Weight:  3_900_000,
			Pool:    "Foundry USA",
			Fee:     &domain.BlockFee{TotalSats: 41_000_000, AvgFeeRate: 31.4},
		}
	}
	priceHistory := make([]float64, 288)
	for i := range priceHistory {
		priceHistory[i] = 64000 + float64(i%50)*10
	}
	histogram := make([][2]float64, 20)
	for i := range histogram {
		histogram[i] = [2]float64{float64(50 - i), float64(10000 * (i + 1))}
	}

	return DashboardData{
		Units:        UnitBTC,
		ProviderName: "mempool.space",
		Network:      "mainnet",
		TipHeight:    912304,
		LastBlockAt:  now.Add(-2 * time.Minute),
		HasPrice:     true,
		Price:        domain.Price{Currency: "USD", Value: 64812.40, Change24h: 2.14},
		PriceHistory: priceHistory,
		Fees:         domain.FeeTiers{HighSatVB: 42, AvgSatVB: 28, LowSatVB: 6, EconomySatVB: 2},
		Mempool:      domain.MempoolState{Count: 8134, VSize: 3_440_000, TotalFeeSats: 29_200_000, Histogram: histogram},
		NextBlocks: []domain.ProjectedBlock{
			{VSize: 960_000, NTx: 3201, TotalFeeSats: 4_120_000, MedianFeeRate: 28, MinFeeRate: 14, MaxFeeRate: 52},
			{VSize: 500_000, NTx: 1800, MedianFeeRate: 22},
			{VSize: 1_000_000, NTx: 3400, MedianFeeRate: 14},
		},
		Blocks:       blocks,
		Cursor:       3,
		Currency:     "USD",
		AssumedVSize: 140,
		Caps:         domain.Capabilities{NextBlockExact: true, MiningPool: true, BlockFees: true},
		FPS:          60,
		Width:        150,
		Height:       45,
	}
}

// TestDashboardRenderSustains60FPS is the integration-style check behind
// the fps counter: if building one dashboard frame ever creeps toward the
// 16.67ms budget a real 60fps loop has, this fails long before a user
// would notice stutter.
func TestDashboardRenderSustains60FPS(t *testing.T) {
	d := representativeDashboard()
	th := theme.Nord

	const warmup = 20
	for range warmup {
		Dashboard(d, th)
	}

	const samples = 200
	start := time.Now()
	for range samples {
		if out := Dashboard(d, th); out == "" {
			t.Fatal("Dashboard() returned empty output")
		}
	}
	elapsed := time.Since(start)
	avg := elapsed / samples

	if avg > frameBudget {
		t.Errorf("average Dashboard() render = %v, want under %v (60fps budget is %v)",
			avg, frameBudget, time.Second/60)
	}
}

// TestDashboardRenderAllGraphStyles checks all three graph styles stay
// within budget too — braille's per-cell dot math is the most expensive.
func TestDashboardRenderAllGraphStyles(t *testing.T) {
	th := theme.Nord
	d := representativeDashboard()
	for _, gs := range []components.GraphStyle{components.GraphBraille, components.GraphBlock, components.GraphTTY} {
		d.GraphStyle = gs
		start := time.Now()
		for range 50 {
			Dashboard(d, th)
		}
		if avg := time.Since(start) / 50; avg > frameBudget {
			t.Errorf("graph style %v: average render = %v, want under %v", gs, avg, frameBudget)
		}
	}
}
