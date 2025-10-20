package memory

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/trace"
)

// AsyncOperationLogger provides structured logging for async memory operations
type AsyncOperationLogger struct {
}

// NewAsyncOperationLogger creates a new async operation logger
func NewAsyncOperationLogger(_ context.Context) *AsyncOperationLogger {
	return &AsyncOperationLogger{}
}

// LogAsyncOperationStart logs the start of an async operation
func (aol *AsyncOperationLogger) LogAsyncOperationStart(
	ctx context.Context,
	operation string,
	memoryID string,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	// Extract trace ID if available
	span := trace.SpanFromContext(ctx)
	traceID := ""
	if span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
	}

	baseFields := []any{
		"operation", operation,
		"memory_id", memoryID,
		"phase", "start",
		"timestamp", time.Now().UTC(),
	}

	if traceID != "" {
		baseFields = append(baseFields, "trace_id", traceID)
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	log.Info("Async operation started", baseFields...)
}

// LogAsyncOperationComplete logs the completion of an async operation
func (aol *AsyncOperationLogger) LogAsyncOperationComplete(
	ctx context.Context,
	operation string,
	memoryID string,
	duration time.Duration,
	err error,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	// Extract trace ID if available
	span := trace.SpanFromContext(ctx)
	traceID := ""
	if span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
	}

	baseFields := []any{
		"operation", operation,
		"memory_id", memoryID,
		"phase", "complete",
		"duration_ms", duration.Milliseconds(),
		"timestamp", time.Now().UTC(),
		"success", err == nil,
	}

	if traceID != "" {
		baseFields = append(baseFields, "trace_id", traceID)
	}

	if err != nil {
		baseFields = append(baseFields, "error", err.Error())
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	if err != nil {
		log.Error("Async operation failed", baseFields...)
	} else {
		log.Info("Async operation completed", baseFields...)
	}
}

// LogFlushOperation logs a flush operation with detailed metadata
func (aol *AsyncOperationLogger) LogFlushOperation(
	ctx context.Context,
	memoryID string,
	flushType string,
	metadata map[string]any,
) {
	baseMetadata := map[string]any{
		"flush_type": flushType,
		"async":      true,
	}
	// Merge with provided metadata
	merged := core.CopyMaps(baseMetadata, metadata)
	aol.LogAsyncOperationStart(ctx, "memory_flush", memoryID, merged)
}

// LogTemporalWorkflow logs Temporal workflow scheduling
func (aol *AsyncOperationLogger) LogTemporalWorkflow(
	ctx context.Context,
	workflowID string,
	workflowType string,
	memoryID string,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	baseFields := []any{
		"workflow_id", workflowID,
		"workflow_type", workflowType,
		"memory_id", memoryID,
		"timestamp", time.Now().UTC(),
		"async", true,
	}

	// Extract trace ID if available
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		baseFields = append(baseFields, "trace_id", span.SpanContext().TraceID().String())
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	log.Info("Temporal workflow scheduled", baseFields...)
}

// LogTokenManagement logs token management operations
func (aol *AsyncOperationLogger) LogTokenManagement(
	ctx context.Context,
	memoryID string,
	operation string,
	tokensUsed int,
	maxTokens int,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	// Calculate usage percentage, avoiding division by zero
	var usagePercentage float64
	if maxTokens > 0 {
		usagePercentage = float64(tokensUsed) / float64(maxTokens) * 100
	}
	baseFields := []any{
		"operation", operation,
		"memory_id", memoryID,
		"tokens_used", tokensUsed,
		"max_tokens", maxTokens,
		"usage_percentage", usagePercentage,
		"timestamp", time.Now().UTC(),
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	switch {
	case tokensUsed > maxTokens:
		log.Warn("Token limit exceeded", baseFields...)
	case maxTokens > 0 && float64(tokensUsed)/float64(maxTokens) > 0.9:
		log.Info("Token usage high", baseFields...)
	default:
		log.Debug("Token management operation", baseFields...)
	}
}

// LogLockOperation logs distributed lock operations
func (aol *AsyncOperationLogger) LogLockOperation(
	ctx context.Context,
	memoryID string,
	operation string,
	success bool,
	duration time.Duration,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	baseFields := []any{
		"operation", operation,
		"memory_id", memoryID,
		"success", success,
		"duration_ms", duration.Milliseconds(),
		"timestamp", time.Now().UTC(),
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	if success {
		log.Debug("Lock operation succeeded", baseFields...)
	} else {
		log.Warn("Lock operation failed", baseFields...)
	}
}

// LogPrivacyOperation logs privacy-related operations
func (aol *AsyncOperationLogger) LogPrivacyOperation(
	ctx context.Context,
	memoryID string,
	operation string,
	metadata map[string]any,
) {
	log := logger.FromContext(ctx)
	baseFields := []any{
		"operation", operation,
		"memory_id", memoryID,
		"privacy", true,
		"timestamp", time.Now().UTC(),
	}

	// Add metadata fields
	for k, v := range metadata {
		baseFields = append(baseFields, k, v)
	}

	log.Info("Privacy operation", baseFields...)
}
