package gateway

import (
	"hash/fnv"
	"strings"
	"sync"
	"time"
)

const maxTrackedIngressBudgets = 8192

type ingressRateBucket struct {
	fingerprint uint64
	started     time.Time
	count       int
	occupied    bool
}

// IngressRateLimiter bounds public requests independently for each endpoint and transport source.
type IngressRateLimiter struct {
	mu      sync.Mutex
	buckets []ingressRateBucket
	maximum int
	window  time.Duration
	now     func() time.Time
}

// NewIngressRateLimiter constructs a bounded fixed-window limiter.
func NewIngressRateLimiter(maximum int, window time.Duration, now func() time.Time) *IngressRateLimiter {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &IngressRateLimiter{
		buckets: make([]ingressRateBucket, maxTrackedIngressBudgets),
		maximum: maximum,
		window:  window,
		now:     now,
	}
}

// Allow consumes one request budget and returns retry guidance when denied.
func (l *IngressRateLimiter) Allow(endpoint string, source string) (bool, time.Duration) {
	if l == nil || l.maximum <= 0 || l.window <= 0 {
		return true, 0
	}
	fingerprint := ingressRateFingerprint(endpoint, source)
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket := &l.buckets[fingerprint%uint64(len(l.buckets))]
	if bucket.occupied && now.Sub(bucket.started) >= l.window {
		*bucket = ingressRateBucket{}
	}
	if !bucket.occupied {
		*bucket = ingressRateBucket{fingerprint: fingerprint, started: now, count: 1, occupied: true}
		return true, 0
	}
	retryAfter := max(l.window-now.Sub(bucket.started), time.Second)
	// A collision must not evict an active counter: doing so would let endpoint
	// churn reset another sender's budget. Fail closed for the remainder of the window.
	if bucket.fingerprint != fingerprint {
		return false, retryAfter
	}
	if bucket.count >= l.maximum {
		return false, retryAfter
	}
	bucket.count++
	return true, 0
}

func ingressRateFingerprint(endpoint string, source string) uint64 {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint == "" {
		normalizedEndpoint = "unknown-endpoint"
	}
	normalizedSource := strings.TrimSpace(source)
	if normalizedSource == "" {
		normalizedSource = "unknown-source"
	}
	hasher := fnv.New64a()
	if _, err := hasher.Write([]byte(normalizedEndpoint)); err != nil {
		panic("hash.Hash.Write must not fail: " + err.Error())
	}
	if _, err := hasher.Write([]byte{0}); err != nil {
		panic("hash.Hash.Write must not fail: " + err.Error())
	}
	if _, err := hasher.Write([]byte(normalizedSource)); err != nil {
		panic("hash.Hash.Write must not fail: " + err.Error())
	}
	return hasher.Sum64()
}
