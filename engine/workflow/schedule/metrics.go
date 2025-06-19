package schedule

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Metrics instruments
	scheduleOperationsTotal    metric.Int64Counter
	scheduledWorkflowsTotal    metric.Int64UpDownCounter
	reconcileDurationHistogram metric.Float64Histogram
	reconcileInflightGauge     metric.Int64UpDownCounter

	// Initialization state
	metricsInitOnce sync.Once
)

// Metrics provides instrumentation for schedule operations
type Metrics struct {
	meter metric.Meter
	log   logger.Logger
}

// NewMetrics creates a new schedule metrics instance
func NewMetrics(ctx context.Context, meter metric.Meter) *Metrics {
	log := logger.FromContext(ctx)
	initMetrics(meter, log)
	return &Metrics{
		meter: meter,
		log:   log,
	}
}

// initMetrics initializes the schedule metrics instruments
func initMetrics(meter metric.Meter, log logger.Logger) {
	if meter == nil {
		return
	}
	metricsInitOnce.Do(func() {
		var err error

		// Schedule operations counter
		scheduleOperationsTotal, err = meter.Int64Counter(
			"compozy_schedule_operations_total",
			metric.WithDescription("Total schedule operations"),
		)
		if err != nil {
			log.Error("Failed to create schedule operations total counter", "error", err)
		}

		// Scheduled workflows gauge
		scheduledWorkflowsTotal, err = meter.Int64UpDownCounter(
			"compozy_scheduled_workflows_total",
			metric.WithDescription("Number of scheduled workflows"),
		)
		if err != nil {
			log.Error("Failed to create scheduled workflows total gauge", "error", err)
		}

		// Reconciliation duration histogram
		reconcileDurationHistogram, err = meter.Float64Histogram(
			"compozy_schedule_reconcile_duration_seconds",
			metric.WithDescription("Schedule reconciliation duration"),
			metric.WithExplicitBucketBoundaries(.1, .25, .5, 1, 2.5, 5, 10, 30, 60),
		)
		if err != nil {
			log.Error("Failed to create reconcile duration histogram", "error", err)
		}

		// Reconciliation in-flight gauge
		reconcileInflightGauge, err = meter.Int64UpDownCounter(
			"compozy_schedule_reconcile_inflight",
			metric.WithDescription("Number of in-flight reconciliation operations"),
		)
		if err != nil {
			log.Error("Failed to create reconcile inflight gauge", "error", err)
		}
	})
}

// RecordOperation records a schedule operation (create, update, delete)
func (m *Metrics) RecordOperation(ctx context.Context, operation, status, project string) {
	if scheduleOperationsTotal == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
		attribute.String("project", project),
	)

	scheduleOperationsTotal.Add(ctx, 1, attrs)
}

// UpdateWorkflowCount updates the scheduled workflows count
func (m *Metrics) UpdateWorkflowCount(ctx context.Context, project, status string, delta int64) {
	if scheduledWorkflowsTotal == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
		attribute.String("status", status),
	)

	scheduledWorkflowsTotal.Add(ctx, delta, attrs)
}

// RecordReconcileDuration records the duration of a reconciliation operation
func (m *Metrics) RecordReconcileDuration(ctx context.Context, project string, duration time.Duration) {
	if reconcileDurationHistogram == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	reconcileDurationHistogram.Record(ctx, duration.Seconds(), attrs)
}

// StartReconciliation marks the start of a reconciliation operation
func (m *Metrics) StartReconciliation(ctx context.Context, project string) {
	if reconcileInflightGauge == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	reconcileInflightGauge.Add(ctx, 1, attrs)
}

// EndReconciliation marks the end of a reconciliation operation
func (m *Metrics) EndReconciliation(ctx context.Context, project string) {
	if reconcileInflightGauge == nil {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("project", project),
	)

	reconcileInflightGauge.Add(ctx, -1, attrs)
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

// ResetMetricsForTesting resets the metrics initialization state for testing
func ResetMetricsForTesting() {
	scheduleOperationsTotal = nil
	scheduledWorkflowsTotal = nil
	reconcileDurationHistogram = nil
	reconcileInflightGauge = nil
	metricsInitOnce = sync.Once{}
}
