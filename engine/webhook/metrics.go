package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	labelUnknownValue   = "unknown"
	outcomeSuccessValue = "success"
	outcomeErrorValue   = "error"
)

// Metrics provides instrumentation for webhook processing
type Metrics struct {
	meter               metric.Meter
	receivedTotal       metric.Int64Counter
	verifiedTotal       metric.Int64Counter
	duplicateTotal      metric.Int64Counter
	dispatchedTotal     metric.Int64Counter
	noMatchTotal        metric.Int64Counter
	failedTotal         metric.Int64Counter
	processingHistogram metric.Float64Histogram
	verifyHistogram     metric.Float64Histogram
	renderHistogram     metric.Float64Histogram
	dispatchHistogram   metric.Float64Histogram
	payloadHistogram    metric.Int64Histogram
	eventOutcomeTotal   metric.Int64Counter
	queueGauge          metric.Int64UpDownCounter
}

// NewMetrics initializes webhook metrics using the provided meter
func NewMetrics(_ context.Context, meter metric.Meter) (*Metrics, error) {
	m := &Metrics{meter: meter}
	if err := m.init(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Metrics) init() error {
	if m.meter == nil {
		return nil
	}
	if err := m.initCounters(); err != nil {
		return err
	}
	return m.initHistograms()
}

func (m *Metrics) initCounters() error {
	counterDefs := []struct {
		target      *metric.Int64Counter
		name        string
		description string
		errLabel    string
	}{
		{&m.receivedTotal, "received_total", "Total webhook requests received", "received"},
		{&m.verifiedTotal, "verified_total", "Total webhook requests successfully verified", "verified"},
		{&m.duplicateTotal, "duplicate_total", "Total duplicate webhook requests detected", "duplicate"},
		{&m.dispatchedTotal, "dispatched_total", "Total webhook events dispatched", "dispatched"},
		{&m.noMatchTotal, "no_match_total", "Total webhook requests with no matching event", "no_match"},
		{&m.failedTotal, "failed_total", "Total webhook processing failures by reason", "failed"},
		{&m.eventOutcomeTotal, "events_total", "Total webhook events received", "events"},
	}
	for _, def := range counterDefs {
		counter, err := m.registerInt64Counter(def.name, def.description, def.errLabel)
		if err != nil {
			return err
		}
		*def.target = counter
	}
	gauge, err := m.registerUpDownCounter(
		"queue_depth",
		"Number of webhook events waiting to be processed",
		"queue depth gauge",
	)
	if err != nil {
		return err
	}
	m.queueGauge = gauge
	return nil
}

// registerInt64Counter creates and names a counter under the webhook subsystem.
func (m *Metrics) registerInt64Counter(name, description, errLabel string) (metric.Int64Counter, error) {
	counter, err := m.meter.Int64Counter(
		monitoringmetrics.MetricNameWithSubsystem("webhook", name),
		metric.WithDescription(description),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook %s counter: %w", errLabel, err)
	}
	return counter, nil
}

// registerUpDownCounter creates a gauge-style counter for queue depth tracking.
func (m *Metrics) registerUpDownCounter(name, description, errLabel string) (metric.Int64UpDownCounter, error) {
	gauge, err := m.meter.Int64UpDownCounter(
		monitoringmetrics.MetricNameWithSubsystem("webhook", name),
		metric.WithDescription(description),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook %s: %w", errLabel, err)
	}
	return gauge, nil
}

func (m *Metrics) initHistograms() error {
	var err error
	m.payloadHistogram, err = m.meter.Int64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("webhook", "payload_size_bytes"),
		metric.WithDescription("Size distribution of webhook payloads"),
		metric.WithUnit("bytes"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook payload histogram: %w", err)
	}
	m.processingHistogram, err = m.meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("webhook", "processing_duration_seconds"),
		metric.WithDescription("Overall webhook processing duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2, 2.5, 5, 10),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook processing duration histogram: %w", err)
	}
	m.verifyHistogram, err = m.meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("webhook", "verify_duration_seconds"),
		metric.WithDescription("Webhook verification duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook verify duration histogram: %w", err)
	}
	m.renderHistogram, err = m.meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("webhook", "render_duration_seconds"),
		metric.WithDescription("Webhook render duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook render duration histogram: %w", err)
	}
	m.dispatchHistogram, err = m.meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem("webhook", "dispatch_duration_seconds"),
		metric.WithDescription("Webhook dispatch duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5, 1),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook dispatch duration histogram: %w", err)
	}
	return nil
}

func (m *Metrics) attrs(slug, workflowID string) metric.MeasurementOption {
	return metric.WithAttributes(attribute.String("slug", slug), attribute.String("workflow_id", workflowID))
}

func (m *Metrics) attrsWithEvent(slug, workflowID, eventName string) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("slug", slug),
		attribute.String("workflow_id", workflowID),
		attribute.String("event_name", eventName),
	)
}

// RecordPayloadSize observes webhook payload sizes grouped by event type and source.
func (m *Metrics) RecordPayloadSize(ctx context.Context, eventType string, source string, payloadBytes int) {
	if m.payloadHistogram == nil || payloadBytes < 0 {
		return
	}
	if eventType == "" {
		eventType = labelUnknownValue
	}
	if source == "" {
		source = labelUnknownValue
	}
	m.payloadHistogram.Record(
		ctx,
		int64(payloadBytes),
		metric.WithAttributes(
			attribute.String("event_type", eventType),
			attribute.String("source", source),
		),
	)
}

// ObserveEventOutcome records processing duration and totals partitioned by outcome.
func (m *Metrics) ObserveEventOutcome(ctx context.Context, eventType string, d time.Duration, outcome string) {
	if eventType == "" {
		eventType = labelUnknownValue
	}
	if outcome != outcomeSuccessValue {
		outcome = outcomeErrorValue
	}
	if m.processingHistogram != nil {
		m.processingHistogram.Record(
			ctx,
			d.Seconds(),
			metric.WithAttributes(
				attribute.String("event_type", eventType),
				attribute.String("outcome", outcome),
			),
		)
	}
	if m.eventOutcomeTotal != nil {
		m.eventOutcomeTotal.Add(
			ctx,
			1,
			metric.WithAttributes(
				attribute.String("event_type", eventType),
				attribute.String("outcome", outcome),
			),
		)
	}
}

// IncrementQueueDepth tracks the number of webhook events awaiting processing.
func (m *Metrics) IncrementQueueDepth(ctx context.Context) {
	if m.queueGauge != nil {
		m.queueGauge.Add(ctx, 1)
	}
}

// DecrementQueueDepth reduces the in-flight webhook processing gauge.
func (m *Metrics) DecrementQueueDepth(ctx context.Context) {
	if m.queueGauge != nil {
		m.queueGauge.Add(ctx, -1)
	}
}

func (m *Metrics) OnReceived(ctx context.Context, slug, workflowID string) {
	if m.receivedTotal != nil {
		m.receivedTotal.Add(ctx, 1, m.attrs(slug, workflowID))
	}
}

func (m *Metrics) OnVerified(ctx context.Context, slug, workflowID string) {
	if m.verifiedTotal != nil {
		m.verifiedTotal.Add(ctx, 1, m.attrs(slug, workflowID))
	}
}

func (m *Metrics) OnDuplicate(ctx context.Context, slug, workflowID string) {
	if m.duplicateTotal != nil {
		m.duplicateTotal.Add(ctx, 1, m.attrs(slug, workflowID))
	}
}

func (m *Metrics) OnDispatched(ctx context.Context, slug, workflowID, eventName string) {
	if m.dispatchedTotal != nil {
		m.dispatchedTotal.Add(ctx, 1, m.attrsWithEvent(slug, workflowID, eventName))
	}
}

func (m *Metrics) OnNoMatch(ctx context.Context, slug, workflowID string) {
	if m.noMatchTotal != nil {
		m.noMatchTotal.Add(ctx, 1, m.attrs(slug, workflowID))
	}
}

func (m *Metrics) OnFailed(ctx context.Context, slug, workflowID, reason string) {
	if m.failedTotal != nil {
		m.failedTotal.Add(
			ctx,
			1,
			metric.WithAttributes(
				attribute.String("slug", slug),
				attribute.String("workflow_id", workflowID),
				attribute.String("reason", core.RedactString(reason)),
			),
		)
	}
}

func (m *Metrics) ObserveOverall(ctx context.Context, slug, workflowID string, d time.Duration) {
	if m.processingHistogram != nil {
		m.processingHistogram.Record(ctx, d.Seconds(), m.attrs(slug, workflowID))
	}
}

func (m *Metrics) ObserveVerify(ctx context.Context, slug, workflowID string, d time.Duration) {
	if m.verifyHistogram != nil {
		m.verifyHistogram.Record(ctx, d.Seconds(), m.attrs(slug, workflowID))
	}
}

func (m *Metrics) ObserveRender(ctx context.Context, slug, workflowID, eventName string, d time.Duration) {
	if m.renderHistogram != nil {
		m.renderHistogram.Record(ctx, d.Seconds(), m.attrsWithEvent(slug, workflowID, eventName))
	}
}

func (m *Metrics) ObserveDispatch(ctx context.Context, slug, workflowID, eventName string, d time.Duration) {
	if m.dispatchHistogram != nil {
		m.dispatchHistogram.Record(ctx, d.Seconds(), m.attrsWithEvent(slug, workflowID, eventName))
	}
}
