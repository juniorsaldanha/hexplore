// Package poll owns every timer and turns provider calls into tea.Msg
// values. Views never call a provider directly (docs/PLAN.md §10) — this is
// what keeps one slow endpoint from blocking the whole UI.
package poll

import (
	"context"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
)

type Intervals struct {
	Tip     time.Duration
	Mempool time.Duration
	Fees    time.Duration
	Price   time.Duration
}

func DefaultIntervals() Intervals {
	return Intervals{
		Tip:     20 * time.Second,
		Mempool: 30 * time.Second,
		Fees:    60 * time.Second,
		Price:   120 * time.Second,
	}
}

// Msg types carry only domain values — poll is on the provider side of the
// dependency rule, but nothing UI-facing should ever see a JSON field name.
type (
	BlocksMsg struct {
		Blocks []domain.Block
		Err    error
	}
	MempoolMsg struct {
		State domain.MempoolState
		Err   error
	}
	FeesMsg struct {
		Tiers domain.FeeTiers
		Err   error
	}
	NextBlocksMsg struct {
		Blocks []domain.ProjectedBlock
		Err    error
	}
	PriceMsg struct {
		Price domain.Price
		Err   error
	}
	PriceHistoryMsg struct {
		Series []float64
		Err    error
	}
)

type Scheduler struct {
	Chain     provider.ChainProvider
	Price     provider.PriceProvider // nil disables the price stream
	NumBlocks int

	Intervals Intervals

	currency atomic.Value // string; set via SetCurrency, read by the price-fetching goroutine

	msgs chan tea.Msg
}

func New(chain provider.ChainProvider, price provider.PriceProvider, currency string, numBlocks int, iv Intervals) *Scheduler {
	s := &Scheduler{
		Chain:     chain,
		Price:     price,
		NumBlocks: numBlocks,
		Intervals: iv,
		msgs:      make(chan tea.Msg, 8),
	}
	s.currency.Store(currency)
	return s
}

// Currency is read by the price-fetching goroutine.
func (s *Scheduler) Currency() string { return s.currency.Load().(string) }

// SetCurrency changes the currency the next price fetch uses — the `p`
// cycle-currency keybinding. Safe to call from the UI goroutine while the
// scheduler's own goroutines are running.
func (s *Scheduler) SetCurrency(currency string) { s.currency.Store(currency) }

// Start launches one goroutine per stream. Each fetches immediately, then on
// every tick thereafter, until ctx is cancelled. When Chain has a live push
// feed (docs/PLAN.md §10), it's layered on top as a lower-latency addition —
// REST polling for blocks/next-blocks/fees keeps running underneath rather
// than standing down, so a feed that eventually dies for good still leaves
// the dashboard updating, just back to poll cadence.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx, s.Intervals.Tip, func() { s.msgs <- s.fetchBlocks(ctx) })
	go s.run(ctx, s.Intervals.Mempool, func() {
		s.msgs <- s.fetchMempool(ctx)
		s.msgs <- s.fetchNextBlocks(ctx)
	})
	go s.run(ctx, s.Intervals.Fees, func() { s.msgs <- s.fetchFees(ctx) })
	if s.Price != nil {
		go s.run(ctx, s.Intervals.Price, func() {
			s.msgs <- s.fetchPrice(ctx)
			s.msgs <- s.fetchPriceHistory(ctx)
		})
	}
	s.startLive(ctx)
}

// startLive is a no-op unless Chain implements provider.LiveChainProvider.
func (s *Scheduler) startLive(ctx context.Context) {
	live, ok := s.Chain.(provider.LiveChainProvider)
	if !ok {
		return
	}
	ch, err := live.Subscribe(ctx)
	if err != nil {
		return
	}
	go func() {
		for ev := range ch {
			if len(ev.Blocks) > 0 {
				s.msgs <- BlocksMsg{Blocks: ev.Blocks}
			}
			if len(ev.NextBlocks) > 0 {
				s.msgs <- NextBlocksMsg{Blocks: ev.NextBlocks}
			}
			if ev.Fees != nil {
				s.msgs <- FeesMsg{Tiers: *ev.Fees}
			}
		}
	}()
}

func (s *Scheduler) run(ctx context.Context, interval time.Duration, fetch func()) {
	fetch()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fetch()
		}
	}
}

// Refresh forces an out-of-band fetch of everything, bypassing the ticker —
// used by the `r` keybinding.
func (s *Scheduler) Refresh(ctx context.Context) {
	go func() {
		s.msgs <- s.fetchBlocks(ctx)
		s.msgs <- s.fetchMempool(ctx)
		s.msgs <- s.fetchNextBlocks(ctx)
		s.msgs <- s.fetchFees(ctx)
		if s.Price != nil {
			s.msgs <- s.fetchPrice(ctx)
			s.msgs <- s.fetchPriceHistory(ctx)
		}
	}()
}

// RefreshPrice forces an out-of-band price fetch — called right after
// SetCurrency so a currency switch doesn't wait for the next tick.
func (s *Scheduler) RefreshPrice(ctx context.Context) {
	if s.Price == nil {
		return
	}
	go func() {
		s.msgs <- s.fetchPrice(ctx)
		s.msgs <- s.fetchPriceHistory(ctx)
	}()
}

// Listen returns a tea.Cmd that yields the next scheduler message. The
// caller must re-issue it after each message to keep listening — the
// standard bubbletea pattern for a single multiplexed channel.
func (s *Scheduler) Listen() tea.Cmd {
	return func() tea.Msg { return <-s.msgs }
}

func (s *Scheduler) fetchBlocks(ctx context.Context) tea.Msg {
	blocks, err := s.Chain.LatestBlocks(ctx, s.NumBlocks)
	return BlocksMsg{Blocks: blocks, Err: err}
}

func (s *Scheduler) fetchMempool(ctx context.Context) tea.Msg {
	m, err := s.Chain.Mempool(ctx)
	return MempoolMsg{State: m, Err: err}
}

func (s *Scheduler) fetchFees(ctx context.Context) tea.Msg {
	f, err := s.Chain.FeeEstimates(ctx)
	return FeesMsg{Tiers: f, Err: err}
}

func (s *Scheduler) fetchNextBlocks(ctx context.Context) tea.Msg {
	b, err := s.Chain.NextBlocks(ctx, 3)
	return NextBlocksMsg{Blocks: b, Err: err}
}

func (s *Scheduler) fetchPrice(ctx context.Context) tea.Msg {
	p, err := s.Price.Spot(ctx, s.Currency())
	return PriceMsg{Price: p, Err: err}
}

func (s *Scheduler) fetchPriceHistory(ctx context.Context) tea.Msg {
	h, err := s.Price.History24h(ctx, s.Currency())
	return PriceHistoryMsg{Series: h, Err: err}
}
