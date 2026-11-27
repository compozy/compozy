package remotehost

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthFailureLimiter(t *testing.T) {
	t.Parallel()

	// Invariant: successful app authentication does not reset failures; owner: remote host limiter.
	t.Run("Should preserve failures while completing successful attempts", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
		limiter := NewAuthFailureLimiter(2, time.Minute, func() time.Time { return now })
		for range 3 {
			if !limiter.Allow("http://app.test", false) {
				t.Fatal("successful attempts exhausted the failure budget")
			}
			limiter.Complete("http://app.test")
		}
		limiter.Failure("http://app.test", false)
		if !limiter.Allow("http://app.test", false) {
			t.Fatal("second attempt was rejected before the failure limit")
		}
		limiter.Complete("http://app.test")
		limiter.Failure("http://app.test", false)
		if limiter.Allow("http://app.test", false) {
			t.Fatal("successful authentication erased a failure in the current window")
		}
		now = now.Add(time.Minute)
		if !limiter.Allow("http://app.test", false) {
			t.Fatal("expired failure window remained blocked")
		}
	})

	t.Run("Should use the system clock when no clock is supplied", func(t *testing.T) {
		t.Parallel()
		limiter := NewAuthFailureLimiter(1, time.Minute, nil)
		limiter.Failure("198.51.100.7", false)
		if limiter.Allow("198.51.100.7", false) {
			t.Fatal("Allow(remote over limit with default clock) = true")
		}
	})

	t.Run("Should rate-limit each remote source while exempting local access (UT-045)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := NewAuthFailureLimiter(2, time.Minute, func() time.Time { return now })
		limiter.Failure("198.51.100.8", false)
		limiter.Failure("198.51.100.8", false)
		if limiter.Allow("198.51.100.8", false) {
			t.Fatal("Allow(remote over limit) = true")
		}
		if !limiter.Allow("198.51.100.9", false) || !limiter.Allow("198.51.100.8", true) {
			t.Fatal("independent remote source or local exemption was denied")
		}
		now = now.Add(2 * time.Minute)
		if !limiter.Allow("198.51.100.8", false) {
			t.Fatal("Allow(remote after window) = false")
		}
	})

	t.Run("Should bound tracked remote sources under distributed probing", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		limiter := NewAuthFailureLimiter(2, time.Minute, func() time.Time { return now })
		for index := range maxTrackedAuthSources {
			limiter.Failure(fmt.Sprintf("source-%d", index), false)
		}
		if limiter.Allow("one-more-source", false) {
			t.Fatal("Allow(new source at tracking capacity) = true")
		}
		if len(limiter.failures) != maxTrackedAuthSources {
			t.Fatalf("tracked sources = %d, want bounded %d", len(limiter.failures), maxTrackedAuthSources)
		}
		now = now.Add(2 * time.Minute)
		if !limiter.Allow("one-more-source", false) {
			t.Fatal("Allow(new source after expiry pruning) = false")
		}
	})

	t.Run("Should reserve the per-source budget across concurrent attempts", func(t *testing.T) {
		t.Parallel()
		const maximum = 2
		limiter := NewAuthFailureLimiter(maximum, time.Minute, nil)
		start := make(chan struct{})
		var allowed atomic.Int64
		var workers sync.WaitGroup
		for range 32 {
			workers.Go(func() {
				<-start
				if limiter.Allow("198.51.100.10", false) {
					allowed.Add(1)
				}
			})
		}
		close(start)
		workers.Wait()
		if got := allowed.Load(); got != maximum {
			t.Fatalf("concurrently allowed attempts = %d, want %d", got, maximum)
		}
	})

	t.Run("Should apply live authentication limits without replacing the limiter", func(t *testing.T) {
		t.Parallel()
		limiter := NewAuthFailureLimiter(2, time.Minute, nil)
		if err := limiter.Reconfigure(1, 2*time.Minute); err != nil {
			t.Fatalf("Reconfigure() error = %v", err)
		}
		if !limiter.Allow("198.51.100.11", false) {
			t.Fatal("Allow(first request after reconfigure) = false")
		}
		if limiter.Allow("198.51.100.11", false) {
			t.Fatal("Allow(second request after reconfigure) = true, want live limit")
		}
	})
}
