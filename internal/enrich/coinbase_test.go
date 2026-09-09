package enrich

import "testing"

func TestSubsidy(t *testing.T) {
	cases := []struct {
		height int
		want   int64
	}{
		{0, 5_000_000_000},
		{209_999, 5_000_000_000},
		{210_000, 2_500_000_000},
		{420_000, 1_250_000_000},
		{210_000*33 - 1, 1}, // last halving before hitting zero
		{210_000 * 34, 0},
	}
	for _, c := range cases {
		if got := Subsidy(c.height); got != c.want {
			t.Errorf("Subsidy(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

func TestCoinbaseFees(t *testing.T) {
	cases := []struct {
		name       string
		voutSum    int64
		height     int
		wantTotal  int64
		wantBurned bool
	}{
		{"normal block, height 0", Subsidy(0) + 12345, 0, 12345, false},
		{"post-halving block", Subsidy(210_000) + 500, 210_000, 500, false},
		{"exact subsidy, zero fees", Subsidy(300_000), 300_000, 0, false},
		{"burned coinbase (like 124724/501726)", Subsidy(124_724) - 1_000_000, 124_724, 0, true},
		{"fully burned", 0, 501_726, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, burned := CoinbaseFees(c.voutSum, c.height)
			if total != c.wantTotal || burned != c.wantBurned {
				t.Errorf("CoinbaseFees(%d, %d) = (%d, %v), want (%d, %v)",
					c.voutSum, c.height, total, burned, c.wantTotal, c.wantBurned)
			}
		})
	}
}

func TestBlockFee(t *testing.T) {
	// 1000 sat total fee, weight 4000 (1000 vB), 101 txs (100 real + coinbase).
	f := BlockFee(Subsidy(500_000)+1000, 500_000, 4000, 101)
	if f.TotalSats != 1000 {
		t.Errorf("TotalSats = %d, want 1000", f.TotalSats)
	}
	if f.AvgFeeRate != 1.0 {
		t.Errorf("AvgFeeRate = %v, want 1.0", f.AvgFeeRate)
	}
	if f.AvgSats != 10 {
		t.Errorf("AvgSats = %d, want 10", f.AvgSats)
	}
	if f.Burned {
		t.Error("Burned = true, want false")
	}

	burned := BlockFee(0, 124_724, 4000, 101)
	if !burned.Burned || burned.TotalSats != 0 {
		t.Errorf("burned block: got %+v", burned)
	}
}
