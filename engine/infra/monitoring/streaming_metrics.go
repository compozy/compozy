package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	streamDurationBuckets = []float64{
		0.5,
		1,
		2,
		5,
		10,
		30,
		60,
		120,
		300,
		600,
	}
	timeToFirstEventBuckets = []float64{
		0.05,
		0.1,
		0.25,
		0.5,
		1,
		2,
		5,
		10,
	}
)

// StreamingMetrics exposes instruments that capture SSE stream lifecycle telemetry.
type StreamingMetrics struct {
	activeStreams    metric.Int64UpDownCounter
	streamDuration   metric.Float64Histogram
	firstEventTiming metric.Float64Histogram
	eventsEmitted    metric.Int64Counter
	streamErrors     metric.Int64Counter
}

func newStreamingMetrics(meter metric.Meter) (*StreamingMetrics, error) {
	if meter == nil {
		return &StreamingMetrics{}, nil
	}
	active, err := createActiveStreamCounter(meter)
	if err != nil {
		return nil, err
	}
	duration, err := createStreamDurationHistogram(meter)
	if err != nil {
		return nil, err
	}
	ttfe, err := createTTFEHistogram(meter)
	if err != nil {
		return nil, err
	}
	events, err := createStreamEventsCounter(meter)
	if err != nil {
		return nil, err
	}
	errorsCounter, err := createStreamErrorsCounter(meter)
	if err != nil {
		return nil, err
	}
	return &StreamingMetrics{
		activeStreams:    active,
		streamDuration:   duration,
		firstEventTiming: ttfe,
		eventsEmitted:    events,
		streamErrors:     errorsCounter,
	}, nil
}

func createActiveStreamCounter(meter metric.Meter) (metric.Int64UpDownCounter, error) {
	counter, err := meter.Int64UpDownCounter(
		metrics.MetricNameWithSubsystem("stream", "active_connections"),
		metric.WithDescription("Active SSE connections grouped by execution kind"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create stream active connections counter: %w", err)
	}
	return counter, nil
}

func createStreamDurationHistogram(meter metric.Meter) (metric.Float64Histogram, error) {
	histogram, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("stream", "connection_duration_seconds"),
		metric.WithDescription("Duration of SSE connections in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(streamDurationBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("create stream duration histogram: %w", err)
	}
	return histogram, nil
}

func createTTFEHistogram(meter metric.Meter) (metric.Float64Histogram, error) {
	histogram, err := meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("stream", "time_to_first_event_seconds"),
		metric.WithDescription("Time between connection acceptance and first event emission"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(timeToFirstEventBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("create time-to-first-event histogram: %w", err)
	}
	return histogram, nil
}

func createStreamEventsCounter(meter metric.Meter) (metric.Int64Counter, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("stream", "events_total"),
		metric.WithDescription("Total SSE events emitted grouped by execution kind and event type"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create stream events counter: %w", err)
	}
	return counter, nil
}

func createStreamErrorsCounter(meter metric.Meter) (metric.Int64Counter, error) {
	counter, err := meter.Int64Counter(
		metrics.MetricNameWithSubsystem("stream", "errors_total"),
		metric.WithDescription("Total SSE stream errors grouped by execution kind and close reason"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create stream errors counter: %w", err)
	}
	return counter, nil
}

// RecordConnect increments the active stream gauge for the provided execution kind.
func (m *StreamingMetrics) RecordConnect(ctx context.Context, kind string) {
	if m == nil || m.activeStreams == nil {
		return
	}
	m.activeStreams.Add(ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
}

// RecordDisconnect decrements the active stream gauge for the provided execution kind.
func (m *StreamingMetrics) RecordDisconnect(ctx context.Context, kind string) {
	if m == nil || m.activeStreams == nil {
		return
	}
	m.activeStreams.Add(ctx, -1, metric.WithAttributes(attribute.String("kind", kind)))
}

// RecordDuration records the stream duration in seconds for the provided execution kind.
func (m *StreamingMetrics) RecordDuration(ctx context.Context, kind string, duration time.Duration) {
	if m == nil || m.streamDuration == nil {
		return
	}
	m.streamDuration.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(attribute.String("kind", kind)),
	)
}

// RecordTimeToFirstEvent captures the elapsed time until the first event is delivered.
func (m *StreamingMetrics) RecordTimeToFirstEvent(ctx context.Context, kind string, latency time.Duration) {
	if m == nil || m.firstEventTiming == nil {
		return
	}
	m.firstEventTiming.Record(
		ctx,
		latency.Seconds(),
		metric.WithAttributes(attribute.String("kind", kind)),
	)
}

// RecordEvent increments the emitted events counter for an execution kind and event type.
func (m *StreamingMetrics) RecordEvent(ctx context.Context, kind, eventType string) {
	if m == nil || m.eventsEmitted == nil {
		return
	}
	m.eventsEmitted.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("event_type", eventType),
		),
	)
}

// RecordError increments the stream error counter with the provided reason.
func (m *StreamingMetrics) RecordError(ctx context.Context, kind, reason string) {
	if m == nil || m.streamErrors == nil {
		return
	}
	m.streamErrors.Add(
		ctx,
		1,
		metric.WithAttributes(
			attribute.String("kind", kind),
			attribute.String("reason", reason),
		),
	)
}
