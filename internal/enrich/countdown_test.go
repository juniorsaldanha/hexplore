package enrich

import (
	"testing"
	"time"
)

func TestBlocksUntilRetarget(t *testing.T) {
	cases := []struct{ height, want int }{
		{0, 2016},
		{1, 2015},
		{2015, 1},
		{2016, 2016},
		{4031, 1},
	}
	for _, c := range cases {
		if got := BlocksUntilRetarget(c.height); got != c.want {
			t.Errorf("BlocksUntilRetarget(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

func TestBlocksUntilHalving(t *testing.T) {
	cases := []struct{ height, want int }{
		{0, 210_000},
		{209_999, 1},
		{210_000, 210_000},
	}
	for _, c := range cases {
		if got := BlocksUntilHalving(c.height); got != c.want {
			t.Errorf("BlocksUntilHalving(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

func TestEstimatedETA(t *testing.T) {
	if got := EstimatedETA(6); got != time.Hour {
		t.Errorf("EstimatedETA(6) = %v, want 1h", got)
	}
}
