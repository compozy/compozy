package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var executionLatencyBuckets = []float64{
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	2.5,
	5,
	10,
	15,
	30,
	60,
	120,
	300,
}

const (
	// ExecutionKindWorkflow records measurements for workflow execution endpoints.
	ExecutionKindWorkflow = "workflow"
	// ExecutionKindAgent records measurements for agent execution endpoints.
	ExecutionKindAgent = "agent"
	// ExecutionKindTask records measurements for task execution endpoints.
	ExecutionKindTask = "task"

	// ExecutionOutcomeSuccess marks successful synchronous executions.
	ExecutionOutcomeSuccess = "success"
	// ExecutionOutcomeError marks synchronous executions that returned an error.
	ExecutionOutcomeError = "error"
	// ExecutionOutcomeTimeout marks synchronous executions that timed out on the server.
	ExecutionOutcomeTimeout = "timeout"
)

// ExecutionMetrics bundles the instruments used by the execution endpoints.
type ExecutionMetrics struct {
	latencyHistogram metric.Float64Histogram
	timeoutCounter   metric.Int64Counter
	errorCounter     metric.Int64Counter
}

func newExecutionMetrics(meter metric.Meter) (*ExecutionMetrics, error) {
	if meter == nil {
		return &ExecutionMetrics{}, nil
	}
	latency, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("http_exec", "sync_latency_seconds"),
		metric.WithDescription("Latency of synchronous execution endpoints"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(executionLatencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution latency histogram: %w", err)
	}
	timeouts, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http_exec", "timeouts_total"),
		metric.WithDescription("Total timeouts observed for execution endpoints"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution timeout counter: %w", err)
	}
	errorsCounter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http_exec", "errors_total"),
		metric.WithDescription("Total errors returned by execution endpoints grouped by HTTP status"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution error counter: %w", err)
	}
	return &ExecutionMetrics{
		latencyHistogram: latency,
		timeoutCounter:   timeouts,
		errorCounter:     errorsCounter,
	}, nil
}

// RecordSyncLatency records the synchronous execution latency with outcome labels.
func (m *ExecutionMetrics) RecordSyncLatency(ctx context.Context, kind, outcome string, duration time.Duration) {
	if m == nil || m.latencyHistogram == nil {
		return
	}
	m.latencyHistogram.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("outcome", outcome),
		),
	)
}

// RecordTimeout increments the timeout counter for a specific execution kind.
func (m *ExecutionMetrics) RecordTimeout(ctx context.Context, kind string) {
	if m == nil || m.timeoutCounter == nil {
		return
	}
	m.timeoutCounter.Add(
		ctx,
		1,
		metric.WithAttributes(attribute.String("kind", kind)),
	)
}

// RecordError increments the error counter labeled by execution kind and HTTP status code.
func (m *ExecutionMetrics) RecordError(ctx context.Context, kind string, statusCode int) {
	if m == nil || m.errorCounter == nil {
		return
	}
	m.errorCounter.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("kind", kind),
			attribute.Int64("code", int64(statusCode)),
		),
	)
}
