package utils

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache(time.Second, 100)
	defer c.Stop()
	c.Set("k", "v")
	v, ok := c.Get("k")
	if !ok || v.(string) != "v" {
		t.Fatal("expected to get cached value")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache(50*time.Millisecond, 100)
	defer c.Stop()
	c.Set("k", "v")
	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("k")
	if ok {
		t.Fatal("expected cache entry to expire")
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(time.Minute, 100)
	defer c.Stop()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	if c.Size() != 0 {
		t.Fatal("expected cache to be empty after clear")
	}
}

func TestCache_MaxEntries(t *testing.T) {
	c := NewCache(time.Minute, 3)
	defer c.Stop()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // should evict oldest
	if c.Size() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.Size())
	}
	// "d" must be present
	if _, ok := c.Get("d"); !ok {
		t.Fatal("expected newest entry to be present")
	}
}

func TestCache_EvictExpired(t *testing.T) {
	c := NewCache(50*time.Millisecond, 100)
	defer c.Stop()
	c.Set("a", 1)
	c.Set("b", 2)
	time.Sleep(120 * time.Millisecond) // wait for evict loop to run
	if c.Size() != 0 {
		t.Fatalf("expected 0 entries after eviction, got %d", c.Size())
	}
}
