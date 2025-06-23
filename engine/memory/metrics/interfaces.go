package metrics

import (
	"context"
	"time"
)

// Provider defines the interface for metrics collection
type Provider interface {
	// RecordOperation records metrics for a memory operation
	RecordOperation(ctx context.Context, operation string, duration time.Duration, attributes map[string]any, err error)
	// RecordCount records a count metric
	RecordCount(ctx context.Context, metric string, value int64, attributes map[string]any)
	// RecordGauge records a gauge metric
	RecordGauge(ctx context.Context, metric string, value float64, attributes map[string]any)
	// RecordHistogram records a histogram metric
	RecordHistogram(ctx context.Context, metric string, value float64, attributes map[string]any)
}

// Collector collects memory-specific metrics
type Collector interface {
	// CollectAppendMetrics collects metrics for append operations
	CollectAppendMetrics(ctx context.Context, duration time.Duration, tokenCount int, messageSize int, err error)
	// CollectReadMetrics collects metrics for read operations
	CollectReadMetrics(ctx context.Context, duration time.Duration, messageCount int, totalTokens int, err error)
	// CollectFlushMetrics collects metrics for flush operations
	CollectFlushMetrics(ctx context.Context, duration time.Duration, messagesFlushed int, tokensFlushed int, err error)
	// CollectMemoryState collects current memory state metrics
	CollectMemoryState(ctx context.Context, tokenCount int, messageCount int, utilizationPercent float64)
}

// Labels defines standard metric labels
type Labels struct {
	ProjectID        string
	MemoryResourceID string
	MemoryInstanceID string
	MemoryType       string
	Operation        string
	ErrorType        string
}
