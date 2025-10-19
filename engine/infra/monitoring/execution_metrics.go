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

var workflowPollDurationBuckets = []float64{
	0.1,
	0.5,
	1,
	2.5,
	5,
	10,
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

	// WorkflowPollOutcomeCompleted records poll metrics for workflows that completed.
	WorkflowPollOutcomeCompleted = "completed"
	// WorkflowPollOutcomeTimeout records poll metrics for workflows that timed out.
	WorkflowPollOutcomeTimeout = "timeout"
	// WorkflowPollOutcomeError records poll metrics for workflows that failed.
	WorkflowPollOutcomeError = "error"
)

// ExecutionMetrics bundles the instruments used by the execution endpoints.
type ExecutionMetrics struct {
	latencyHistogram    metric.Float64Histogram
	timeoutCounter      metric.Int64Counter
	errorCounter        metric.Int64Counter
	asyncStartedCounter metric.Int64Counter
	workflowPollCounter metric.Int64Counter
	workflowPollTimer   metric.Float64Histogram
}

func newExecutionMetrics(meter metric.Meter) (*ExecutionMetrics, error) {
	if meter == nil {
		return &ExecutionMetrics{}, nil
	}
	latency, err := createSyncLatencyHistogram(meter)
	if err != nil {
		return nil, err
	}
	timeouts, err := createExecutionTimeoutCounter(meter)
	if err != nil {
		return nil, err
	}
	errorsCounter, err := createExecutionErrorCounter(meter)
	if err != nil {
		return nil, err
	}
	asyncStarted, err := createAsyncExecutionCounter(meter)
	if err != nil {
		return nil, err
	}
	workflowPolls, workflowPollDuration, err := createWorkflowPollMetrics(meter)
	if err != nil {
		return nil, err
	}
	return &ExecutionMetrics{
		latencyHistogram:    latency,
		timeoutCounter:      timeouts,
		errorCounter:        errorsCounter,
		asyncStartedCounter: asyncStarted,
		workflowPollCounter: workflowPolls,
		workflowPollTimer:   workflowPollDuration,
	}, nil
}

func createSyncLatencyHistogram(meter metric.Meter) (metric.Float64Histogram, error) {
	histogram, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("http_exec", "sync_latency_seconds"),
		metric.WithDescription("Latency of synchronous execution endpoints"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(executionLatencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution latency histogram: %w", err)
	}
	return histogram, nil
}

func createExecutionTimeoutCounter(meter metric.Meter) (metric.Int64Counter, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http_exec", "timeouts_total"),
		metric.WithDescription("Total timeouts observed for execution endpoints"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution timeout counter: %w", err)
	}
	return counter, nil
}

func createExecutionErrorCounter(meter metric.Meter) (metric.Int64Counter, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http_exec", "errors_total"),
		metric.WithDescription("Total errors returned by execution endpoints grouped by HTTP status"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution error counter: %w", err)
	}
	return counter, nil
}

func createAsyncExecutionCounter(meter metric.Meter) (metric.Int64Counter, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("http_exec", "started_total"),
		metric.WithDescription("Total async execution starts accepted"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create async execution started counter: %w", err)
	}
	return counter, nil
}

func createWorkflowPollMetrics(meter metric.Meter) (metric.Int64Counter, metric.Float64Histogram, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("workflow", "sync_polls_total"),
		metric.WithDescription("Total poll iterations per workflow sync execution"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create workflow poll counter: %w", err)
	}
	histogram, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("workflow", "sync_poll_duration_seconds"),
		metric.WithDescription("Total time spent polling for workflow completion"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(workflowPollDurationBuckets...),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create workflow poll duration histogram: %w", err)
	}
	return counter, histogram, nil
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

// RecordAsyncStarted increments the counter tracking accepted asynchronous executions.
func (m *ExecutionMetrics) RecordAsyncStarted(ctx context.Context, kind string) {
	if m == nil || m.asyncStartedCounter == nil {
		return
	}
	m.asyncStartedCounter.Add(
		ctx,
		1,
		metric.WithAttributes(attribute.String("kind", kind)),
	)
}

// RecordWorkflowPolls records the total poll attempts for a sync workflow execution with outcome labels.
// Note: workflow_id is intentionally omitted from labels to avoid high cardinality.
// Use logs or traces to track individual workflow execution details.
func (m *ExecutionMetrics) RecordWorkflowPolls(ctx context.Context, _ string, attempts int, outcome string) {
	if m == nil || m.workflowPollCounter == nil {
		return
	}
	if attempts < 0 {
		attempts = 0
	}
	m.workflowPollCounter.Add(
		ctx,
		int64(attempts),
		metric.WithAttributes(
			attribute.String("outcome", outcome),
		),
	)
}

// RecordWorkflowPollDuration records the total time spent polling a sync workflow execution.
// Note: workflow_id is intentionally omitted from labels to avoid high cardinality.
// Use logs or traces to track individual workflow execution details.
func (m *ExecutionMetrics) RecordWorkflowPollDuration(ctx context.Context, _ string, duration time.Duration) {
	if m == nil || m.workflowPollTimer == nil {
		return
	}
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	m.workflowPollTimer.Record(ctx, seconds)
}
