package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics provides instrumentation for webhook processing
type Metrics struct {
	meter               metric.Meter
	log                 logger.Logger
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
}

// NewMetrics initializes webhook metrics using the provided meter
func NewMetrics(ctx context.Context, meter metric.Meter) (*Metrics, error) {
	log := logger.FromContext(ctx)
	m := &Metrics{meter: meter, log: log}
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
	var err error
	m.receivedTotal, err = m.meter.Int64Counter(
		"compozy_webhook_received_total",
		metric.WithDescription("Total webhook requests received"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook received counter: %w", err)
	}
	m.verifiedTotal, err = m.meter.Int64Counter(
		"compozy_webhook_verified_total",
		metric.WithDescription("Total webhook requests successfully verified"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook verified counter: %w", err)
	}
	m.duplicateTotal, err = m.meter.Int64Counter(
		"compozy_webhook_duplicate_total",
		metric.WithDescription("Total duplicate webhook requests detected"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook duplicate counter: %w", err)
	}
	m.dispatchedTotal, err = m.meter.Int64Counter(
		"compozy_webhook_dispatched_total",
		metric.WithDescription("Total webhook events dispatched"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook dispatched counter: %w", err)
	}
	m.noMatchTotal, err = m.meter.Int64Counter(
		"compozy_webhook_no_match_total",
		metric.WithDescription("Total webhook requests with no matching event"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook no_match counter: %w", err)
	}
	m.failedTotal, err = m.meter.Int64Counter(
		"compozy_webhook_failed_total",
		metric.WithDescription("Total webhook processing failures by reason"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook failed counter: %w", err)
	}
	return nil
}

func (m *Metrics) initHistograms() error {
	var err error
	m.processingHistogram, err = m.meter.Float64Histogram(
		"compozy_webhook_processing_duration_seconds",
		metric.WithDescription("Overall webhook processing duration"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook processing duration histogram: %w", err)
	}
	m.verifyHistogram, err = m.meter.Float64Histogram(
		"compozy_webhook_verify_duration_seconds",
		metric.WithDescription("Webhook verification duration"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook verify duration histogram: %w", err)
	}
	m.renderHistogram, err = m.meter.Float64Histogram(
		"compozy_webhook_render_duration_seconds",
		metric.WithDescription("Webhook render duration"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook render duration histogram: %w", err)
	}
	m.dispatchHistogram, err = m.meter.Float64Histogram(
		"compozy_webhook_dispatch_duration_seconds",
		metric.WithDescription("Webhook dispatch duration"),
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
				attribute.String("reason", reason),
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
