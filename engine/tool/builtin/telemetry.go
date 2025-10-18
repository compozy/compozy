package builtin

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	initMetricsOnce         sync.Once
	metricsResetMu          sync.Mutex
	toolInvocationsTotal    metric.Int64Counter
	toolLatencySeconds      metric.Float64Histogram
	toolResponseBytes       metric.Int64Histogram
	toolStepExecutionsTotal metric.Int64Counter
	toolStepLatencySeconds  metric.Float64Histogram
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
			metrics.MetricNameWithSubsystem("tool", "invocations_total"),
			metric.WithDescription("Total cp__ tool invocations grouped by status"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolLatencySeconds, err = meter.Float64Histogram(
			metrics.MetricNameWithSubsystem("tool", "latency_seconds"),
			metric.WithDescription("cp__ tool invocation latency in seconds"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolResponseBytes, err = meter.Int64Histogram(
			metrics.MetricNameWithSubsystem("tool", "response_bytes"),
			metric.WithDescription("cp__ tool response size in bytes"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolStepExecutionsTotal, err = meter.Int64Counter(
			metrics.MetricNameWithSubsystem("tool", "step_executions_total"),
			metric.WithDescription("Total cp__ tool step executions grouped by status"),
		)
		if err != nil {
			initErr = err
			return
		}
		toolStepLatencySeconds, err = meter.Float64Histogram(
			metrics.MetricNameWithSubsystem("tool", "step_latency_seconds"),
			metric.WithDescription("cp__ tool step execution latency in seconds"),
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

// RecordStep records metrics for an individual orchestrated step using the
// shared builtin telemetry instruments. Additional attributes may be provided
// to enrich the metric with contextual details (e.g., agent identifiers or
// parallel execution flags).
func RecordStep(
	ctx context.Context,
	toolID string,
	stepType string,
	status string,
	duration time.Duration,
	attrs ...attribute.KeyValue,
) {
	attributes := []attribute.KeyValue{
		attribute.String("tool_id", toolID),
		attribute.String("step_type", stepType),
		attribute.String("status", status),
	}
	if len(attrs) > 0 {
		attributes = append(attributes, attrs...)
	}
	if toolStepExecutionsTotal != nil {
		toolStepExecutionsTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
	}
	if toolStepLatencySeconds != nil {
		toolStepLatencySeconds.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	}
}

// ResetMetricsForTesting clears the cached telemetry instruments so tests can
// reinitialize metrics state in isolation.
func ResetMetricsForTesting() {
	metricsResetMu.Lock()
	defer metricsResetMu.Unlock()
	initMetricsOnce = sync.Once{}
	toolInvocationsTotal = nil
	toolLatencySeconds = nil
	toolResponseBytes = nil
	toolStepExecutionsTotal = nil
	toolStepLatencySeconds = nil
}
