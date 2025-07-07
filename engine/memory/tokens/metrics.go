package tokens

import (
	"sync/atomic"
	"time"
)

// TokenMetrics tracks token counting performance
type TokenMetrics struct {
	successCount  atomic.Uint64
	errorCount    atomic.Uint64
	droppedCount  atomic.Uint64
	totalDuration atomic.Uint64
	countDuration atomic.Uint64
}

// NewTokenMetrics creates a new TokenMetrics instance
func NewTokenMetrics() *TokenMetrics {
	return &TokenMetrics{}
}

func (tm *TokenMetrics) IncrementSuccess() {
	tm.successCount.Add(1)
}

func (tm *TokenMetrics) IncrementErrors() {
	tm.errorCount.Add(1)
}

func (tm *TokenMetrics) IncrementDropped() {
	tm.droppedCount.Add(1)
}

func (tm *TokenMetrics) RecordDuration(d time.Duration) {
	// Safe conversion - only positive durations should be recorded
	if d >= 0 {
		nanos := d.Nanoseconds()
		// Ensure we don't overflow when converting int64 to uint64
		if nanos >= 0 {
			tm.totalDuration.Add(uint64(nanos))
			tm.countDuration.Add(1)
		}
	}
}

func (tm *TokenMetrics) GetStats() map[string]any {
	count := tm.countDuration.Load()
	avgDuration := time.Duration(0)
	if count > 0 {
		// Safe conversion back to int64 for time.Duration
		totalNanos := tm.totalDuration.Load()
		avgNanos := totalNanos / count
		// avgNanos is guaranteed to be <= totalNanos, which fits in uint64
		// and avgNanos/count will be much smaller, so it's safe to convert to int64
		avgDuration = time.Duration(int64(avgNanos)) //nolint:gosec // avgNanos is within int64 range
	}
	return map[string]any{
		"success_count":   tm.successCount.Load(),
		"error_count":     tm.errorCount.Load(),
		"dropped_count":   tm.droppedCount.Load(),
		"avg_duration_ns": avgDuration.Nanoseconds(),
		"avg_duration_ms": avgDuration.Milliseconds(),
	}
}
