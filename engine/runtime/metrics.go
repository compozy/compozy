package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type toolExecutionOutcome string

const (
	outcomeSuccess toolExecutionOutcome = "success"
	outcomeError   toolExecutionOutcome = "error"
	outcomeTimeout toolExecutionOutcome = "timeout"
)

type toolErrorKind string

const (
	errorKindStart   toolErrorKind = "start"
	errorKindStdin   toolErrorKind = "stdin"
	errorKindStdout  toolErrorKind = "stdout"
	errorKindStderr  toolErrorKind = "stderr"
	errorKindWait    toolErrorKind = "wait"
	errorKindParse   toolErrorKind = "parse"
	errorKindTimeout toolErrorKind = "timeout"
	errorKindUnknown toolErrorKind = "unknown"
)

type toolProcessStatus string

const (
	processStatusExit   toolProcessStatus = "exit"
	processStatusSignal toolProcessStatus = "signal"
)

type runtimeMetrics struct {
	initOnce sync.Once

	executionLatency metric.Float64Histogram
	errorCounter     metric.Int64Counter
	timeoutCounter   metric.Int64Counter
	processExits     metric.Int64Counter
	outputSize       metric.Float64Histogram
}

var metricsContainer runtimeMetrics

func metricsRecorder() *runtimeMetrics {
	metricsContainer.initOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.runtime")
		metricsContainer.executionLatency = createRuntimeHistogram(
			meter,
			monitoringmetrics.MetricNameWithSubsystem("runtime", "tool_execute_seconds"),
			"Latency of runtime tool executions from start to completion",
			"s",
			[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		)
		metricsContainer.errorCounter = createRuntimeCounter(
			meter,
			monitoringmetrics.MetricNameWithSubsystem("runtime", "tool_errors_total"),
			"Total runtime tool errors categorized by failure point",
		)
		metricsContainer.timeoutCounter = createRuntimeCounter(
			meter,
			monitoringmetrics.MetricNameWithSubsystem("runtime", "tool_timeouts_total"),
			"Total tool executions that exceeded timeout",
		)
		metricsContainer.processExits = createRuntimeCounter(
			meter,
			monitoringmetrics.MetricNameWithSubsystem("runtime", "bun_process_exits_total"),
			"Bun process termination reasons",
		)
		metricsContainer.outputSize = createRuntimeHistogram(
			meter,
			monitoringmetrics.MetricNameWithSubsystem("runtime", "tool_output_bytes"),
			"Size distribution of tool stdout payloads",
			"By",
			[]float64{100, 1000, 10000, 100000, 1000000, 10000000},
		)
	})
	return &metricsContainer
}

func createRuntimeHistogram(
	meter metric.Meter,
	name string,
	description string,
	unit string,
	boundaries []float64,
) metric.Float64Histogram {
	histogram, err := meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
		metric.WithExplicitBucketBoundaries(boundaries...),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create runtime histogram %s: %w", name, err))
	}
	return histogram
}

func createRuntimeCounter(
	meter metric.Meter,
	name string,
	description string,
) metric.Int64Counter {
	counter, err := meter.Int64Counter(
		name,
		metric.WithDescription(description),
		metric.WithUnit("1"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create runtime counter %s: %w", name, err))
	}
	return counter
}

func recordToolExecution(ctx context.Context, toolID string, duration time.Duration, outcome toolExecutionOutcome) {
	recorder := metricsRecorder()
	if recorder.executionLatency == nil {
		return
	}
	recorder.executionLatency.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("tool_id", toolID),
			attribute.String("outcome", string(outcome)),
		),
	)
}

func recordToolError(ctx context.Context, toolID string, kind toolErrorKind) {
	recorder := metricsRecorder()
	if recorder.errorCounter == nil {
		return
	}
	recorder.errorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("tool_id", toolID),
			attribute.String("error_kind", string(kind)),
		),
	)
}

func recordToolTimeout(ctx context.Context, toolID string) {
	recorder := metricsRecorder()
	if recorder.timeoutCounter == nil {
		return
	}
	recorder.timeoutCounter.Add(ctx, 1,
		metric.WithAttributes(attribute.String("tool_id", toolID)),
	)
}

func recordProcessExit(ctx context.Context, status toolProcessStatus, code int, signal string) {
	recorder := metricsRecorder()
	if recorder.processExits == nil {
		return
	}
	recorder.processExits.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", string(status)),
			attribute.Int("code", code),
			attribute.String("signal", signal),
		),
	)
}

func recordToolOutputSize(ctx context.Context, toolID string, sizeBytes int) {
	recorder := metricsRecorder()
	if recorder.outputSize == nil {
		return
	}
	recorder.outputSize.Record(ctx, float64(sizeBytes),
		metric.WithAttributes(attribute.String("tool_id", toolID)),
	)
}

type toolError struct {
	kind toolErrorKind
	err  error
}

func (e *toolError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *toolError) Unwrap() error {
	return e.err
}

func (e *toolError) Kind() toolErrorKind {
	return e.kind
}

func wrapToolError(err error, kind toolErrorKind) error {
	if err == nil {
		return nil
	}
	var existing *toolError
	if errors.As(err, &existing) {
		return err
	}
	return &toolError{kind: kind, err: err}
}

func extractToolErrorKind(err error) (toolErrorKind, bool) {
	var te *toolError
	if errors.As(err, &te) {
		return te.Kind(), true
	}
	return "", false
}
