// Package cache holds the two lifetimes hexplore's data comes in: an
// immutable store for confirmed chain data that never needs to expire, and
// a short-TTL store for tip/mempool/fee/price data that goes stale fast.
// Both are in-memory only for now — bbolt persistence across restarts is a
// later phase (docs/PLAN.md §10).
package cache

import (
	"sync"
	"time"
)

// TTL is a generic, goroutine-safe cache where each entry expires after a
// fixed duration from when it was set.
type TTL[K comparable, V any] struct {
	mu      sync.Mutex
	entries map[K]ttlEntry[V]
}

type ttlEntry[V any] struct {
	value   V
	expires time.Time
}

func NewTTL[K comparable, V any]() *TTL[K, V] {
	return &TTL[K, V]{entries: make(map[K]ttlEntry[V])}
}

func (c *TTL[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *TTL[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = ttlEntry[V]{value: value, expires: time.Now().Add(ttl)}
}

// Immutable is a goroutine-safe store for values that, once set, never
// change — confirmed blocks and transactions, coinbase-derived fee totals.
type Immutable[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]V
}

func NewImmutable[K comparable, V any]() *Immutable[K, V] {
	return &Immutable[K, V]{entries: make(map[K]V)}
}

func (c *Immutable[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *Immutable[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
}
