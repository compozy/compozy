package bridgesdk

import (
	"testing"
	"time"
)

func TestDedupCacheSuppressesDuplicatesWithinTTLAndReleasesAfterExpiry(t *testing.T) {
	t.Parallel()

	t.Run("Should suppress duplicates within TTL and release after expiry", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		cache := NewDedupCache(time.Minute, 10)
		cache.now = func() time.Time { return now }

		if duplicate := cache.Mark("dup-key"); duplicate {
			t.Fatal("first Mark() = duplicate, want false")
		}
		if duplicate := cache.Mark("dup-key"); !duplicate {
			t.Fatal("second Mark() = false, want true")
		}

		now = now.Add(2 * time.Minute)
		if duplicate := cache.Mark("dup-key"); duplicate {
			t.Fatal("Mark() after expiry = true, want false")
		}
	})
}

func TestDedupCacheClearDropsTrackedKeys(t *testing.T) {
	t.Parallel()

	t.Run("Should drop tracked keys after clear", func(t *testing.T) {
		t.Parallel()

		cache := NewDedupCache(time.Minute, 10)
		cache.Mark("dup-key")
		cache.Clear()

		if duplicate := cache.Mark("dup-key"); duplicate {
			t.Fatal("Mark() after Clear() = true, want false")
		}
	})
}

func TestNewDedupCacheAppliesDefaultBounds(t *testing.T) {
	t.Parallel()

	t.Run("Should apply default TTL and max size when bounds are empty", func(t *testing.T) {
		t.Parallel()

		cache := NewDedupCache(0, 0)
		if cache.ttl != defaultDedupTTL {
			t.Fatalf("cache.ttl = %s, want %s", cache.ttl, defaultDedupTTL)
		}
		if cache.maxSize != defaultDedupMaxSize {
			t.Fatalf("cache.maxSize = %d, want %d", cache.maxSize, defaultDedupMaxSize)
		}
	})
}

func TestDedupCacheEnforcesMaxSize(t *testing.T) {
	t.Parallel()

	t.Run("Should evict the oldest fresh key when max size is exceeded", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		cache := NewDedupCache(time.Hour, 2)
		cache.now = func() time.Time { return now }

		cache.Mark("first")
		now = now.Add(time.Second)
		cache.Mark("second")
		now = now.Add(time.Second)
		cache.Mark("third")

		if got := len(cache.seen); got != 2 {
			t.Fatalf("len(cache.seen) = %d, want 2", got)
		}
		if cache.Seen("first") {
			t.Fatal("Seen(first) = true, want false")
		}
		if !cache.Seen("second") {
			t.Fatal("Seen(second) = false, want true")
		}
		if !cache.Seen("third") {
			t.Fatal("Seen(third) = false, want true")
		}
	})
}
