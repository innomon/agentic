package server

import (
	"context"
	"sync"
	"time"
)

// DedupCache is a time-bounded deduplication cache.
type DedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

// NewDedupCache creates a new dedup cache with the given TTL.
func NewDedupCache(ttl time.Duration) *DedupCache {
	return &DedupCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// Check returns true if the key has been seen within the TTL.
func (d *DedupCache) Check(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.entries[key]
	if !ok {
		return false
	}
	return time.Since(t) < d.ttl
}

// Record records a key in the cache.
func (d *DedupCache) Record(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[key] = time.Now()
}

// Cleanup removes expired entries. Should be called periodically.
func (d *DedupCache) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for k, t := range d.entries {
		if now.Sub(t) >= d.ttl {
			delete(d.entries, k)
		}
	}
}

// StartCleanupLoop starts a background goroutine that cleans up expired entries.
func (d *DedupCache) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.Cleanup()
			}
		}
	}()
}
