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
	if d >= 0 {
		nanos := d.Nanoseconds()
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
		totalNanos := tm.totalDuration.Load()
		avgNanos := totalNanos / count
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
