package mempoolspace

import (
	"testing"
	"time"
)

func TestReconnectDelayGrowsAndCaps(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{6, 64 * time.Second}, // 2^6=64s, but that's still capped below
	}
	for _, c := range cases {
		got := reconnectDelay(c.attempt)
		want := min(c.want, liveReconnectMax)
		if got != want {
			t.Errorf("reconnectDelay(%d) = %v, want %v", c.attempt, got, want)
		}
	}
	// Very large attempt counts must never exceed the cap or overflow.
	if got := reconnectDelay(1000); got != liveReconnectMax {
		t.Errorf("reconnectDelay(1000) = %v, want capped at %v", got, liveReconnectMax)
	}
}

func TestDecodeLiveEventBlocksReversedToNewestFirst(t *testing.T) {
	// mempool.space sends "blocks" oldest-first; decodeLiveEvent must
	// reverse it so callers get the same newest-first order as every REST
	// endpoint (LatestBlocks, etc.).
	data := []byte(`{"blocks":[
		{"id":"aaa","height":100},
		{"id":"bbb","height":101},
		{"id":"ccc","height":102}
	]}`)
	ev, ok := decodeLiveEvent(data)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(ev.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(ev.Blocks))
	}
	if ev.Blocks[0].Height != 102 || ev.Blocks[2].Height != 100 {
		t.Fatalf("blocks not reversed to newest-first: heights = [%d,%d,%d]",
			ev.Blocks[0].Height, ev.Blocks[1].Height, ev.Blocks[2].Height)
	}
}

func TestDecodeLiveEventMempoolBlocks(t *testing.T) {
	data := []byte(`{"mempool-blocks":[
		{"blockVSize":997983.75,"nTx":6146,"totalFees":739611,"medianFee":0.37,"feeRange":[0.3,50.3]}
	]}`)
	ev, ok := decodeLiveEvent(data)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(ev.NextBlocks) != 1 {
		t.Fatalf("got %d next blocks, want 1", len(ev.NextBlocks))
	}
	if ev.NextBlocks[0].TotalFeeSats != 739611 {
		t.Errorf("TotalFeeSats = %d, want 739611", ev.NextBlocks[0].TotalFeeSats)
	}
	if ev.NextBlocks[0].Approx {
		t.Error("native mempool-blocks data should not be marked Approx")
	}
}

func TestDecodeLiveEventFees(t *testing.T) {
	data := []byte(`{"fees":{"fastestFee":5.582,"halfHourFee":4.772,"hourFee":3.277,"economyFee":0.2,"minimumFee":0.1}}`)
	ev, ok := decodeLiveEvent(data)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.Fees == nil {
		t.Fatal("expected Fees to be set")
	}
	if ev.Fees.HighSatVB != 5.582 || ev.Fees.LowSatVB != 3.277 {
		t.Errorf("Fees = %+v", ev.Fees)
	}
}

func TestDecodeLiveEventCombinedMessage(t *testing.T) {
	// The server sometimes bundles several keys into one message (observed
	// live: blocks + mempool-blocks + fees + mempoolInfo + da together).
	data := []byte(`{
		"blocks":[{"id":"aaa","height":1}],
		"mempool-blocks":[{"nTx":1,"totalFees":100,"medianFee":1,"feeRange":[1,1]}],
		"fees":{"fastestFee":1,"halfHourFee":1,"hourFee":1,"economyFee":1,"minimumFee":1},
		"mempoolInfo":{"loaded":true,"size":1000},
		"vBytesPerSecond":666
	}`)
	ev, ok := decodeLiveEvent(data)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(ev.Blocks) != 1 || len(ev.NextBlocks) != 1 || ev.Fees == nil {
		t.Fatalf("expected all three fields populated, got %+v", ev)
	}
}

func TestDecodeLiveEventIrrelevantMessageIsSkipped(t *testing.T) {
	// Only mempoolInfo/vBytesPerSecond/da — nothing this provider maps to a
	// domain event yet.
	data := []byte(`{"mempoolInfo":{"loaded":true,"size":1000},"vBytesPerSecond":666}`)
	_, ok := decodeLiveEvent(data)
	if ok {
		t.Error("expected ok=false for a message with no mapped fields")
	}
}

func TestDecodeLiveEventGarbageIsSkipped(t *testing.T) {
	_, ok := decodeLiveEvent([]byte(`not json`))
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
	_, ok = decodeLiveEvent([]byte(`{}`))
	if ok {
		t.Error("expected ok=false for an empty object")
	}
}
