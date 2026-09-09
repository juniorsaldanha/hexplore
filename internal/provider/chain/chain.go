// Package chain is the fallback chain (docs/PLAN.md §2) — the part that
// makes hexplore durable against a single free host changing its rate
// limits or terms. It wraps an ordered list of ChainProviders and tries
// them in preference order, skipping hosts in cooldown after a failure.
package chain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/juniorsaldanha/hexplore/internal/domain"
	"github.com/juniorsaldanha/hexplore/internal/provider"
	"github.com/juniorsaldanha/hexplore/internal/provider/esplora"
)

const maxCooldown = 5 * time.Minute

type hostState struct {
	provider            provider.ChainProvider
	consecutiveFailures int
	cooldownUntil       time.Time
}

// Chain implements provider.ChainProvider by trying each host in order,
// skipping ones still in cooldown. Name() and Caps() report whichever host
// actually served the last request, so the UI degrades live instead of
// lying about what the active host can do.
type Chain struct {
	mu     sync.Mutex
	hosts  []*hostState
	active int
	pinned bool
}

var (
	_ provider.ChainProvider     = (*Chain)(nil)
	_ provider.LiveChainProvider = (*Chain)(nil)
)

func New(hosts ...provider.ChainProvider) *Chain {
	states := make([]*hostState, len(hosts))
	for i, h := range hosts {
		states[i] = &hostState{provider: h}
	}
	return &Chain{hosts: states}
}

// Pin fixes the active host by a case-insensitive substring match on its
// Name() (e.g. "blockstream") and disables failover — the `:provider <name>`
// command. Unpin re-enables it.
func (c *Chain) Pin(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, h := range c.hosts {
		if strings.Contains(strings.ToLower(h.provider.Name()), strings.ToLower(name)) {
			c.active = i
			c.pinned = true
			return nil
		}
	}
	return fmt.Errorf("chain: no host matching %q", name)
}

func (c *Chain) Unpin() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pinned = false
}

func (c *Chain) Pinned() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pinned
}

// Degraded reports whether the currently active host is not the first
// (most preferred) one — the status bar's cue to show "degraded".
func (c *Chain) Degraded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active != 0
}

// Hosts lists every configured host's name, in preference order.
func (c *Chain) Hosts() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.hosts))
	for i, h := range c.hosts {
		names[i] = h.provider.Name()
	}
	return names
}

func (c *Chain) Name() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hosts[c.active].provider.Name()
}

func (c *Chain) Caps() domain.Capabilities {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hosts[c.active].provider.Caps()
}

func backoff(consecutiveFailures int) time.Duration {
	d := time.Second * time.Duration(1<<consecutiveFailures)
	if d > maxCooldown || d <= 0 {
		return maxCooldown
	}
	return d
}

// isRetryable reports whether err is a host-health problem (rate limit,
// server error, network failure) worth failing over, versus a legitimate
// application-level error (not found, unsupported) that should propagate.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, provider.ErrUnsupported) {
		return false
	}
	if httpErr, ok := errors.AsType[*esplora.HTTPError](err); ok {
		return httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	return true // network-level error: timeout, connection refused, DNS, etc.
}

// withFailover tries each unpinned, non-cooldown host in preference order,
// updating c.active to whichever host actually answered. The mutex is only
// held around c's own state (active index, cooldowns) — never during fn
// itself, since a slow call (the WebSocket-backed ProjectedBlockTxs can take
// several seconds) would otherwise stall every other chain call behind it.
// c.hosts itself is never mutated after New(), so ranging over it needs no
// lock; only each hostState's mutable fields and c.active/c.pinned do.
func withFailover[T any](c *Chain, fn func(provider.ChainProvider) (T, error)) (T, error) {
	var zero T

	c.mu.Lock()
	pinned, pinnedHost := c.pinned, c.hosts[c.active].provider
	c.mu.Unlock()
	if pinned {
		return fn(pinnedHost)
	}

	var lastErr error
	now := time.Now()
	for i, h := range c.hosts {
		c.mu.Lock()
		cooldownUntil := h.cooldownUntil
		c.mu.Unlock()
		if now.Before(cooldownUntil) {
			continue
		}

		v, err := fn(h.provider)
		if err == nil {
			c.mu.Lock()
			h.consecutiveFailures = 0
			c.active = i
			c.mu.Unlock()
			return v, nil
		}
		lastErr = err
		if !isRetryable(err) {
			c.mu.Lock()
			c.active = i
			c.mu.Unlock()
			return zero, err
		}
		c.mu.Lock()
		h.consecutiveFailures++
		h.cooldownUntil = now.Add(backoff(h.consecutiveFailures))
		c.mu.Unlock()
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("chain: all hosts in cooldown")
	}
	return zero, lastErr
}

func (c *Chain) TipHeight(ctx context.Context) (int, error) {
	return withFailover(c, func(p provider.ChainProvider) (int, error) { return p.TipHeight(ctx) })
}

func (c *Chain) LatestBlocks(ctx context.Context, n int) ([]domain.Block, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]domain.Block, error) { return p.LatestBlocks(ctx, n) })
}

func (c *Chain) BlockByHeight(ctx context.Context, height int) (domain.Block, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.Block, error) { return p.BlockByHeight(ctx, height) })
}

func (c *Chain) BlockByHash(ctx context.Context, hash string) (domain.Block, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.Block, error) { return p.BlockByHash(ctx, hash) })
}

func (c *Chain) BlockTxs(ctx context.Context, hash string, start int) ([]domain.Tx, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]domain.Tx, error) { return p.BlockTxs(ctx, hash, start) })
}

func (c *Chain) Tx(ctx context.Context, txid string) (domain.Tx, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.Tx, error) { return p.Tx(ctx, txid) })
}

func (c *Chain) Address(ctx context.Context, addr string) (domain.Address, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.Address, error) { return p.Address(ctx, addr) })
}

func (c *Chain) AddressTxs(ctx context.Context, addr string, lastSeenTxID string) ([]domain.Tx, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]domain.Tx, error) { return p.AddressTxs(ctx, addr, lastSeenTxID) })
}

func (c *Chain) AddressPrefix(ctx context.Context, prefix string) ([]string, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]string, error) { return p.AddressPrefix(ctx, prefix) })
}

func (c *Chain) Mempool(ctx context.Context) (domain.MempoolState, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.MempoolState, error) { return p.Mempool(ctx) })
}

func (c *Chain) FeeEstimates(ctx context.Context) (domain.FeeTiers, error) {
	return withFailover(c, func(p provider.ChainProvider) (domain.FeeTiers, error) { return p.FeeEstimates(ctx) })
}

func (c *Chain) NextBlocks(ctx context.Context, n int) ([]domain.ProjectedBlock, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]domain.ProjectedBlock, error) { return p.NextBlocks(ctx, n) })
}

func (c *Chain) ProjectedBlockTxs(ctx context.Context, blockIndex int) ([]domain.ProjectedTx, error) {
	return withFailover(c, func(p provider.ChainProvider) ([]domain.ProjectedTx, error) {
		return p.ProjectedBlockTxs(ctx, blockIndex)
	})
}

// Subscribe implements provider.LiveChainProvider by delegating to whichever
// host is active right now — chosen once, not re-evaluated on failover. If
// that host can't push (e.g. plain Esplora), the caller falls back to REST
// polling for everything, same as if no live feed existed at all.
func (c *Chain) Subscribe(ctx context.Context) (<-chan provider.LiveEvent, error) {
	c.mu.Lock()
	active := c.hosts[c.active].provider
	c.mu.Unlock()
	live, ok := active.(provider.LiveChainProvider)
	if !ok {
		return nil, fmt.Errorf("chain: active host %q has no live feed: %w", active.Name(), provider.ErrUnsupported)
	}
	return live.Subscribe(ctx)
}
