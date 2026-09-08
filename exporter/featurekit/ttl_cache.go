package featurekit

import (
	"sync"
	"sync/atomic"
	"time"
)

// TTLCache is a small thread-safe in-memory cache for feature-owned lookup
// results. A non-positive default TTL disables Set; use SetWithTTL when entries
// need value-specific TTLs.
type TTLCache[K comparable, V any] struct {
	mu    sync.RWMutex
	ttl   time.Duration
	now   func() time.Time
	items map[K]ttlCacheItem[V]

	hits    atomic.Uint64
	misses  atomic.Uint64
	sets    atomic.Uint64
	deletes atomic.Uint64
	expired atomic.Uint64
	clears  atomic.Uint64
}

type ttlCacheItem[V any] struct {
	value     V
	expiresAt time.Time
}

type TTLCacheStats struct {
	Entries uint64
	Hits    uint64
	Misses  uint64
	Sets    uint64
	Deletes uint64
	Expired uint64
	Clears  uint64
}

func NewTTLCache[K comparable, V any](ttl time.Duration) *TTLCache[K, V] {
	return newTTLCacheWithClock[K, V](ttl, time.Now)
}

func newTTLCacheWithClock[K comparable, V any](ttl time.Duration, now func() time.Time) *TTLCache[K, V] {
	if now == nil {
		now = time.Now
	}
	return &TTLCache[K, V]{
		ttl:   ttl,
		now:   now,
		items: make(map[K]ttlCacheItem[V]),
	}
}

func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	var zero V
	if c == nil {
		return zero, false
	}

	c.mu.RLock()
	item, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		c.misses.Add(1)
		return zero, false
	}
	if c.now().Before(item.expiresAt) {
		c.mu.RUnlock()
		c.hits.Add(1)
		return item.value, true
	}
	c.mu.RUnlock()

	c.mu.Lock()
	if current, ok := c.items[key]; ok && current.expiresAt.Equal(item.expiresAt) {
		delete(c.items, key)
		c.expired.Add(1)
	}
	c.mu.Unlock()
	c.misses.Add(1)
	return zero, false
}

func (c *TTLCache[K, V]) Set(key K, value V) {
	if c == nil {
		return
	}
	c.SetWithTTL(key, value, c.ttl)
}

func (c *TTLCache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	if c == nil || ttl <= 0 {
		return
	}

	c.mu.Lock()
	c.items[key] = ttlCacheItem[V]{
		value:     value,
		expiresAt: c.now().Add(ttl),
	}
	c.mu.Unlock()
	c.sets.Add(1)
}

func (c *TTLCache[K, V]) Delete(key K) {
	if c == nil {
		return
	}

	c.mu.Lock()
	if _, ok := c.items[key]; ok {
		delete(c.items, key)
		c.deletes.Add(1)
	}
	c.mu.Unlock()
}

func (c *TTLCache[K, V]) Clear() {
	if c == nil {
		return
	}

	c.mu.Lock()
	clear(c.items)
	c.mu.Unlock()
	c.clears.Add(1)
}

func (c *TTLCache[K, V]) Len() int {
	if c == nil {
		return 0
	}

	now := c.now()
	expired := 0
	c.mu.Lock()
	for key, item := range c.items {
		if !now.Before(item.expiresAt) {
			delete(c.items, key)
			expired++
		}
	}
	size := len(c.items)
	c.mu.Unlock()
	if expired > 0 {
		c.expired.Add(uint64(expired))
	}
	return size
}

func (c *TTLCache[K, V]) Stats() TTLCacheStats {
	if c == nil {
		return TTLCacheStats{}
	}

	return TTLCacheStats{
		Entries: uint64(c.Len()),
		Hits:    c.hits.Load(),
		Misses:  c.misses.Load(),
		Sets:    c.sets.Load(),
		Deletes: c.deletes.Load(),
		Expired: c.expired.Load(),
		Clears:  c.clears.Load(),
	}
}
