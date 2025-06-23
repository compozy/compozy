package instance

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/memory/metrics"
)

// MetricsCollector implements the Metrics interface for memory operations
type MetricsCollector struct {
	provider metrics.Provider
	labels   metrics.Labels
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(provider metrics.Provider, labels *metrics.Labels) *MetricsCollector {
	return &MetricsCollector{
		provider: provider,
		labels:   *labels,
	}
}

// RecordAppend records metrics for an append operation
func (mc *MetricsCollector) RecordAppend(ctx context.Context, duration time.Duration, tokenCount int, err error) {
	attributes := mc.buildAttributes("append")
	if err != nil {
		attributes["error"] = err.Error()
		attributes["error_type"] = getErrorType(err)
	}

	// Record operation duration
	mc.provider.RecordOperation(ctx, "memory.append", duration, attributes, err)

	// Record token count if successful
	if err == nil && tokenCount > 0 {
		mc.provider.RecordHistogram(ctx, "memory.tokens.appended", float64(tokenCount), attributes)
	}
}

// RecordRead records metrics for a read operation
func (mc *MetricsCollector) RecordRead(
	ctx context.Context,
	duration time.Duration,
	messageCount int,
	_ int, // totalTokens - unused
	err error,
) {
	attributes := mc.buildAttributes("read")
	if err != nil {
		attributes["error"] = err.Error()
		attributes["error_type"] = getErrorType(err)
	}

	// Record operation duration
	mc.provider.RecordOperation(ctx, "memory.read", duration, attributes, err)

	// Record message count if successful
	if err == nil {
		mc.provider.RecordHistogram(ctx, "memory.messages.read", float64(messageCount), attributes)
	}
}

// RecordFlush records metrics for a flush operation
func (mc *MetricsCollector) RecordFlush(
	ctx context.Context,
	duration time.Duration,
	messagesFlushed int,
	tokensFlushed int,
	err error,
) {
	attributes := mc.buildAttributes("flush")
	if err != nil {
		attributes["error"] = err.Error()
		attributes["error_type"] = getErrorType(err)
	}

	// Record operation duration
	mc.provider.RecordOperation(ctx, "memory.flush", duration, attributes, err)

	// Record messages flushed if successful
	if err == nil {
		if messagesFlushed > 0 {
			mc.provider.RecordHistogram(ctx, "memory.messages.flushed", float64(messagesFlushed), attributes)
		}
		if tokensFlushed > 0 {
			mc.provider.RecordHistogram(ctx, "memory.tokens.flushed", float64(tokensFlushed), attributes)
		}
	}
}

// RecordTokenCount records the current token count
func (mc *MetricsCollector) RecordTokenCount(ctx context.Context, count int) {
	attributes := mc.buildAttributes("gauge")
	mc.provider.RecordGauge(ctx, "memory.tokens.current", float64(count), attributes)
}

// RecordMessageCount records the current message count
func (mc *MetricsCollector) RecordMessageCount(ctx context.Context, count int) {
	attributes := mc.buildAttributes("gauge")
	mc.provider.RecordGauge(ctx, "memory.messages.current", float64(count), attributes)
}

// buildAttributes builds the base attributes for metrics
func (mc *MetricsCollector) buildAttributes(operation string) map[string]any {
	return map[string]any{
		"project_id":         mc.labels.ProjectID,
		"memory_resource_id": mc.labels.MemoryResourceID,
		"memory_instance_id": mc.labels.MemoryInstanceID,
		"memory_type":        mc.labels.MemoryType,
		"operation":          operation,
	}
}

// getErrorType extracts a normalized error type from an error
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Map specific errors to types
	switch err {
	case ErrFlushAlreadyPending:
		return "flush_already_pending"
	default:
		// Extract from error message or use generic
		return "unknown"
	}
}
