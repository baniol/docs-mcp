package utils

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// Cache is a simple TTL cache safe for concurrent use.
type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		ttl: ttl,
		m:   make(map[string]cacheEntry),
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
	c.mu.Unlock()
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
