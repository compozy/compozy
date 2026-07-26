package bridgesdk

import (
	"strings"
	"sync"
	"time"
)

const (
	defaultDedupTTL     = 5 * time.Minute
	defaultDedupMaxSize = 2000
)

// DedupCache is the adapter-local TTL cache used to suppress immediate platform retries.
type DedupCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	now     func() time.Time
	seen    map[string]time.Time
}

// NewDedupCache constructs a TTL-based dedup cache.
func NewDedupCache(ttl time.Duration, maxSize int) *DedupCache {
	if ttl <= 0 {
		ttl = defaultDedupTTL
	}
	if maxSize <= 0 {
		maxSize = defaultDedupMaxSize
	}
	return &DedupCache{
		ttl:     ttl,
		maxSize: maxSize,
		now: func() time.Time {
			return time.Now().UTC()
		},
		seen: make(map[string]time.Time, maxSize),
	}
}

// Mark returns true when the idempotency key is already active within the TTL window.
func (c *DedupCache) Mark(key string) bool {
	if c == nil {
		return false
	}

	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.evictExpiredLocked(now)
	if seenAt, ok := c.seen[trimmedKey]; ok && now.Sub(seenAt) < c.ttl {
		return true
	}

	c.seen[trimmedKey] = now
	if len(c.seen) > c.maxSize {
		c.evictExpiredLocked(now)
		c.evictOldestLocked()
	}
	return false
}

// Seen reports whether the idempotency key is already active within the TTL window without recording it.
func (c *DedupCache) Seen(key string) bool {
	if c == nil {
		return false
	}

	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.evictExpiredLocked(now)
	seenAt, ok := c.seen[trimmedKey]
	return ok && now.Sub(seenAt) < c.ttl
}

// Clear removes every tracked idempotency key.
func (c *DedupCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seen = make(map[string]time.Time, c.maxSize)
}

func (c *DedupCache) evictExpiredLocked(now time.Time) {
	cutoff := now.Add(-c.ttl)
	for key, seenAt := range c.seen {
		if seenAt.After(cutoff) {
			continue
		}
		delete(c.seen, key)
	}
}

func (c *DedupCache) evictOldestLocked() {
	for len(c.seen) > c.maxSize {
		var oldestKey string
		var oldestAt time.Time
		found := false
		for key, seenAt := range c.seen {
			if !found || seenAt.Before(oldestAt) || (seenAt.Equal(oldestAt) && key < oldestKey) {
				oldestKey = key
				oldestAt = seenAt
				found = true
			}
		}
		if !found {
			return
		}
		delete(c.seen, oldestKey)
	}
}
