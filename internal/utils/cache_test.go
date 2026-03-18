package utils

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache(time.Second)
	c.Set("k", "v")
	v, ok := c.Get("k")
	if !ok || v.(string) != "v" {
		t.Fatal("expected to get cached value")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache(50 * time.Millisecond)
	c.Set("k", "v")
	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("k")
	if ok {
		t.Fatal("expected cache entry to expire")
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	if c.Size() != 0 {
		t.Fatal("expected cache to be empty after clear")
	}
}
