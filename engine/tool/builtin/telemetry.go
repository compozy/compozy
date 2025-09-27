package builtin

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	initMetricsOnce      sync.Once
	toolInvocationsTotal metric.Int64Counter
	toolLatencySeconds   metric.Float64Histogram
	toolResponseBytes    metric.Int64Histogram
)

const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// InitMetrics registers OpenTelemetry instruments for builtin tools.
func InitMetrics(meter metric.Meter) error {
	if meter == nil {
		return nil
	}
	var initErr error
	initMetricsOnce.Do(func() {
		var err error
		toolInvocationsTotal, err = meter.Int64Counter(
			"compozy_tool_invocations_total",
			metric.WithDescription("Total cp__ tool invocations grouped by status"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolLatencySeconds, err = meter.Float64Histogram(
			"compozy_tool_latency_seconds",
			metric.WithDescription("cp__ tool invocation latency in seconds"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolResponseBytes, err = meter.Int64Histogram(
			"compozy_tool_response_bytes",
			metric.WithDescription("cp__ tool response size in bytes"),
		)
		if err != nil {
			initErr = err
			return
		}
	})
	return initErr
}

// RequestIDFromContext returns the request id from context if present; empty otherwise.
func RequestIDFromContext(ctx context.Context) string {
	id, err := core.GetRequestID(ctx)
	if err != nil {
		return ""
	}
	return id
}

// RecordInvocation records standard metrics for a builtin tool invocation.
// status should be "success" or "failure"; errorCode is optional.
func RecordInvocation(
	ctx context.Context,
	toolID string,
	requestID string,
	status string,
	duration time.Duration,
	responseBytes int,
	errorCode string,
) {
	if toolInvocationsTotal != nil {
		attrs := []attribute.KeyValue{
			attribute.String("tool_id", toolID),
			attribute.String("status", status),
		}
		if errorCode != "" {
			attrs = append(attrs, attribute.String("error_code", errorCode))
		}
		toolInvocationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if toolLatencySeconds != nil {
		toolLatencySeconds.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String("tool_id", toolID),
			attribute.String("status", status),
		))
	}
	if responseBytes > 0 && toolResponseBytes != nil {
		toolResponseBytes.Record(ctx, int64(responseBytes), metric.WithAttributes(
			attribute.String("tool_id", toolID),
		))
	}
	// best-effort logging of anomalies
	if status == StatusFailure {
		logger.FromContext(ctx).
			Debug("cp__ tool failure recorded", "tool_id", toolID, "request_id", requestID, "error_code", errorCode)
	}
}
