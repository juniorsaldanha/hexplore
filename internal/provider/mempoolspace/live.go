package mempoolspace

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

var _ provider.LiveChainProvider = (*Provider)(nil)

// liveEnvelope covers the /v1/ws push shapes this provider turns into
// domain events — the same JSON shapes as the REST endpoints, verified
// against the live feed (docs: https://mempool.space/docs/api/websocket).
// mempoolInfo/vBytesPerSecond/da also arrive but have no domain.MempoolState
// equivalent (no vsize or fee histogram) and are left unused; Mempool stays
// on its own REST poll.
type liveEnvelope struct {
	Blocks        []v1BlockJSON        `json:"blocks"`
	MempoolBlocks []v1MempoolBlockJSON `json:"mempool-blocks"`
	Fees          *v1FeesJSON          `json:"fees"`
}

const (
	liveReconnectBase = time.Second
	liveReconnectMax  = 60 * time.Second
)

// reconnectDelay is exponential backoff capped at liveReconnectMax —
// attempt 0 (immediately after a successful connection drops) waits 1s,
// doubling each further consecutive failure.
func reconnectDelay(attempt int) time.Duration {
	return min(liveReconnectBase*time.Duration(int64(1)<<min(attempt, 6)), liveReconnectMax)
}

// Subscribe implements provider.LiveChainProvider.
func (p *Provider) Subscribe(ctx context.Context) (<-chan provider.LiveEvent, error) {
	ch := make(chan provider.LiveEvent, 4)
	go p.runLive(ctx, ch)
	return ch, nil
}

// runLive reconnects forever (capped exponential backoff) until ctx is
// cancelled — a WS drop that outlasts the process should degrade to
// REST-only, not silently stop trying.
func (p *Provider) runLive(ctx context.Context, ch chan<- provider.LiveEvent) {
	defer close(ch)
	target, err := wsURL(p.Provider.BaseURL)
	if err != nil {
		return
	}

	attempt := 0
	for ctx.Err() == nil {
		if err := p.liveOnce(ctx, target, ch); err == nil {
			attempt = 0
		} else {
			attempt++
		}
		if ctx.Err() != nil {
			return
		}
		delay := reconnectDelay(attempt)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// liveOnce holds one connection open until it errors or ctx is done.
func (p *Provider) liveOnce(ctx context.Context, target string, ch chan<- provider.LiveEvent) error {
	conn, _, err := websocket.Dial(ctx, target, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	want := `{"action":"want","data":["blocks","mempool-blocks","stats"]}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(want)); err != nil {
		return err
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		ev, ok := decodeLiveEvent(data)
		if !ok {
			continue
		}
		select {
		case ch <- ev:
		case <-ctx.Done():
			return nil
		}
	}
}

func decodeLiveEvent(data []byte) (provider.LiveEvent, bool) {
	var env liveEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return provider.LiveEvent{}, false
	}
	var ev provider.LiveEvent
	got := false

	if len(env.Blocks) > 0 {
		blocks := make([]domain.Block, len(env.Blocks))
		for i, b := range env.Blocks {
			blocks[i] = b.toDomain()
		}
		// mempool.space sends these oldest-first; every REST endpoint (and
		// every caller of LatestBlocks) expects newest-first.
		for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
			blocks[i], blocks[j] = blocks[j], blocks[i]
		}
		ev.Blocks = blocks
		got = true
	}
	if len(env.MempoolBlocks) > 0 {
		nb := make([]domain.ProjectedBlock, len(env.MempoolBlocks))
		for i, b := range env.MempoolBlocks {
			nb[i] = b.toDomain()
		}
		ev.NextBlocks = nb
		got = true
	}
	if env.Fees != nil {
		f := env.Fees.toDomain()
		ev.Fees = &f
		got = true
	}
	return ev, got
}
