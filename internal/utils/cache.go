package utils

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// Cache is a TTL cache with max entry limit, safe for concurrent use.
// Expired entries are evicted proactively by a background goroutine.
type Cache struct {
	mu         sync.RWMutex
	ttl        time.Duration
	maxEntries int
	m          map[string]cacheEntry
	stop       chan struct{}
}

// NewCache creates a cache with the given TTL and max entry limit.
// A background goroutine evicts expired entries every TTL interval.
// Call Stop() to release the goroutine.
func NewCache(ttl time.Duration, maxEntries int) *Cache {
	c := &Cache{
		ttl:        ttl,
		maxEntries: maxEntries,
		m:          make(map[string]cacheEntry),
		stop:       make(chan struct{}),
	}
	go c.evictLoop()
	return c
}

func (c *Cache) evictLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stop:
			return
		}
	}
}

func (c *Cache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.m {
		if now.After(e.expiresAt) {
			delete(c.m, k)
		}
	}
	c.mu.Unlock()
}

// Stop shuts down the background eviction goroutine.
func (c *Cache) Stop() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	c.m[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	if len(c.m) > c.maxEntries {
		c.evictOldestLocked()
	}
	c.mu.Unlock()
}

// evictOldestLocked removes the entry closest to expiration. Must be called with mu held.
func (c *Cache) evictOldestLocked() {
	var oldestKey string
	var oldestExp time.Time
	first := true
	for k, e := range c.m {
		if first || e.expiresAt.Before(oldestExp) {
			oldestKey = k
			oldestExp = e.expiresAt
			first = false
		}
	}
	if !first {
		delete(c.m, oldestKey)
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.m = make(map[string]cacheEntry)
	c.mu.Unlock()
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}
