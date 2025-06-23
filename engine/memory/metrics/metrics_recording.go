package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordMemoryMessage records metrics for messages added to memory
func RecordMemoryMessage(ctx context.Context, memoryID string, projectID string, tokens int64) {
	if memoryMessagesTotal == nil {
		return
	}
	attributes := []attribute.KeyValue{
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
	}
	memoryMessagesTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
	if tokens > 0 && memoryTokensTotal != nil {
		memoryTokensTotal.Add(ctx, tokens, metric.WithAttributes(attributes...))
	}
}

// RecordMemoryTrim records metrics for memory trim operations
func RecordMemoryTrim(ctx context.Context, memoryID string, projectID string, strategy string, tokensSaved int64) {
	if memoryTrimTotal == nil {
		return
	}
	attributes := []attribute.KeyValue{
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
		attribute.String("strategy", strategy),
	}
	memoryTrimTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
	if tokensSaved > 0 && memoryTokensSavedTotal != nil {
		memoryTokensSavedTotal.Add(ctx, tokensSaved, metric.WithAttributes(attributes...))
	}
}

// RecordMemoryFlush records metrics for memory flush operations
func RecordMemoryFlush(ctx context.Context, memoryID string, projectID string, flushType string) {
	if memoryFlushTotal == nil {
		return
	}
	memoryFlushTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
		attribute.String("flush_type", flushType),
	))
}

// RecordMemoryLockAcquire records metrics for memory lock acquisitions
func RecordMemoryLockAcquire(ctx context.Context, memoryID string, projectID string) {
	if memoryLockAcquireTotal == nil {
		return
	}
	memoryLockAcquireTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
	))
}

// RecordMemoryLockContention records metrics for memory lock contentions
func RecordMemoryLockContention(ctx context.Context, memoryID string, projectID string) {
	if memoryLockContentionTotal == nil {
		return
	}
	memoryLockContentionTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
	))
}

// RecordMemoryOp records metrics for memory operations with latency
func RecordMemoryOp(
	ctx context.Context,
	memoryID string,
	projectID string,
	operation string,
	latency time.Duration,
	tokenCount int64,
	err error,
) {
	if memoryOperationLatency == nil {
		return
	}
	status := "success"
	if err != nil {
		status = "error"
	}
	attributes := []attribute.KeyValue{
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
		attribute.String("operation", operation),
		attribute.String("status", status),
	}
	if tokenCount > 0 {
		attributes = append(attributes, attribute.Int64("token_count", tokenCount))
	}
	memoryOperationLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attributes...))
}

// RecordTemporalActivity records metrics for Temporal activities
func RecordTemporalActivity(ctx context.Context, memoryID string, activityType string, projectID string) {
	if memoryTemporalActivities == nil {
		return
	}
	memoryTemporalActivities.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("activity_type", activityType),
		attribute.String("project_id", projectID),
	))
}

// RecordConfigResolution records metrics for config resolutions
func RecordConfigResolution(ctx context.Context, pattern string, projectID string) {
	if memoryConfigResolution == nil {
		return
	}
	memoryConfigResolution.Add(ctx, 1, metric.WithAttributes(
		attribute.String("pattern", pattern),
		attribute.String("project_id", projectID),
	))
}

// RecordPrivacyExclusion records metrics for privacy exclusions
func RecordPrivacyExclusion(ctx context.Context, memoryID string, reason string, projectID string) {
	if memoryPrivacyExclusions == nil {
		return
	}
	memoryPrivacyExclusions.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("reason", reason),
		attribute.String("project_id", projectID),
	))
}

// RecordRedactionOperation records metrics for redaction operations
func RecordRedactionOperation(ctx context.Context, memoryID string, fieldCount int64, projectID string) {
	if memoryRedactionOperations == nil {
		return
	}
	memoryRedactionOperations.Add(ctx, fieldCount, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
	))
}

// RecordCircuitBreakerTrip records metrics for circuit breaker trips
func RecordCircuitBreakerTrip(ctx context.Context, memoryID string, projectID string) {
	if memoryCircuitBreakerTrips == nil {
		return
	}
	memoryCircuitBreakerTrips.Add(ctx, 1, metric.WithAttributes(
		attribute.String("memory_id", memoryID),
		attribute.String("project_id", projectID),
	))
}
