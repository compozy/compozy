package schedule

import (
	"context"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics provides instrumentation for schedule operations
type Metrics struct {
	meter                      metric.Meter
	log                        logger.Logger
	scheduleOperationsTotal    metric.Int64Counter
	scheduledWorkflowsTotal    metric.Int64UpDownCounter
	reconcileDurationHistogram metric.Float64Histogram
	reconcileInflightGauge     metric.Int64UpDownCounter
}

// NewMetrics creates a new schedule metrics instance
func NewMetrics(ctx context.Context, meter metric.Meter) *Metrics {
	log := logger.FromContext(ctx)
	m := &Metrics{
		meter: meter,
		log:   log,
	}
	m.initMetrics()
	return m
}

// initMetrics initializes the schedule metrics instruments for this instance
func (m *Metrics) initMetrics() {
	if m.meter == nil {
		return
	}
	var err error

	// Schedule operations counter
	m.scheduleOperationsTotal, err = m.meter.Int64Counter(
		"compozy_schedule_operations_total",
		metric.WithDescription("Total schedule operations"),
	)
	if err != nil {
		m.log.Error("Failed to create schedule operations total counter", "error", err)
	}

	// Scheduled workflows gauge
	m.scheduledWorkflowsTotal, err = m.meter.Int64UpDownCounter(
		"compozy_scheduled_workflows_total",
		metric.WithDescription("Number of scheduled workflows"),
	)
	if err != nil {
		m.log.Error("Failed to create scheduled workflows total gauge", "error", err)
	}

	// Reconciliation duration histogram
	m.reconcileDurationHistogram, err = m.meter.Float64Histogram(
		"compozy_schedule_reconcile_duration_seconds",
		metric.WithDescription("Schedule reconciliation duration"),
		metric.WithExplicitBucketBoundaries(.1, .25, .5, 1, 2.5, 5, 10, 30, 60),
	)
	if err != nil {
		m.log.Error("Failed to create reconcile duration histogram", "error", err)
	}

	// Reconciliation in-flight gauge
	m.reconcileInflightGauge, err = m.meter.Int64UpDownCounter(
		"compozy_schedule_reconcile_inflight",
		metric.WithDescription("Number of in-flight reconciliation operations"),
	)
	if err != nil {
		m.log.Error("Failed to create reconcile inflight gauge", "error", err)
	}
}

// RecordOperation records a schedule operation (create, update, delete)
func (m *Metrics) RecordOperation(ctx context.Context, operation, status, project string) {
	if m.scheduleOperationsTotal == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
		attribute.String("project", project),
	)

	m.scheduleOperationsTotal.Add(ctx, 1, attrs)
}

// UpdateWorkflowCount updates the scheduled workflows count
func (m *Metrics) UpdateWorkflowCount(ctx context.Context, project, status string, delta int64) {
	if m.scheduledWorkflowsTotal == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
		attribute.String("status", status),
	)

	m.scheduledWorkflowsTotal.Add(ctx, delta, attrs)
}

// RecordReconcileDuration records the duration of a reconciliation operation
func (m *Metrics) RecordReconcileDuration(ctx context.Context, project string, duration time.Duration) {
	if m.reconcileDurationHistogram == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	m.reconcileDurationHistogram.Record(ctx, duration.Seconds(), attrs)
}

// StartReconciliation marks the start of a reconciliation operation
func (m *Metrics) StartReconciliation(ctx context.Context, project string) {
	if m.reconcileInflightGauge == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	m.reconcileInflightGauge.Add(ctx, 1, attrs)
}

// EndReconciliation marks the end of a reconciliation operation
func (m *Metrics) EndReconciliation(ctx context.Context, project string) {
	if m.reconcileInflightGauge == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	m.reconcileInflightGauge.Add(ctx, -1, attrs)
}

// ReconciliationTracker helps track reconciliation lifecycle
type ReconciliationTracker struct {
	metrics   *Metrics
	ctx       context.Context
	project   string
	startTime time.Time
}

// NewReconciliationTracker creates a new reconciliation tracker
func (m *Metrics) NewReconciliationTracker(ctx context.Context, project string) *ReconciliationTracker {
	tracker := &ReconciliationTracker{
		metrics:   m,
		ctx:       ctx,
		project:   project,
		startTime: time.Now(),
	}

	// Mark reconciliation as started
	m.StartReconciliation(ctx, project)

	return tracker
}

// Finish completes the reconciliation tracking
func (t *ReconciliationTracker) Finish() {
	// Record duration
	duration := time.Since(t.startTime)
	t.metrics.RecordReconcileDuration(t.ctx, t.project, duration)

	// Mark reconciliation as ended
	t.metrics.EndReconciliation(t.ctx, t.project)
}
