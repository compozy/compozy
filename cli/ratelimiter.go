package cli

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/time/rate"
)

// RateLimiter provides token bucket rate limiting for API requests.
type RateLimiter struct {
	limiter *rate.Limiter
	mu      sync.RWMutex
}

// NewRateLimiter creates a new rate limiter based on the configuration.
func NewRateLimiter(limit int64, period time.Duration) *RateLimiter {
	if limit <= 0 || period <= 0 {
		return nil
	}
	// Calculate rate per second
	ratePerSecond := float64(limit) / period.Seconds()
	// Use limit as burst size to allow full quota immediately
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(ratePerSecond), int(limit)),
	}
}

// Allow checks if a request is allowed under the current rate limit.
func (r *RateLimiter) Allow() bool {
	if r == nil {
		return true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limiter.Allow()
}

// Wait blocks until a request is allowed or the context is canceled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limiter.Wait(ctx)
}

// Reserve returns a reservation for n tokens.
func (r *RateLimiter) Reserve() *rate.Reservation {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limiter.Reserve()
}

// TokensAt returns the number of tokens available at time t.
func (r *RateLimiter) TokensAt(t time.Time) float64 {
	if r == nil {
		return float64(^uint(0) >> 1) // Max int
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limiter.TokensAt(t)
}

// rateLimitMiddleware returns a resty middleware function for rate limiting.
func rateLimitMiddleware(rl *RateLimiter) func(*resty.Client, *resty.Request) error {
	return func(_ *resty.Client, req *resty.Request) error {
		if rl == nil {
			return nil
		}
		ctx := req.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		// Wait for rate limiter approval
		if err := rl.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}
		return nil
	}
}
