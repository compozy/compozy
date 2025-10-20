package interceptor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	workflowStartedTotal       metric.Int64Counter
	workflowCompletedTotal     metric.Int64Counter
	workflowFailedTotal        metric.Int64Counter
	workflowTaskDuration       metric.Float64Histogram
	workersRunning             metric.Int64UpDownCounter
	workersConfigured          metric.Int64ObservableGauge
	configuredWorkerCountValue atomic.Int64
	callbackRegistration       metric.Registration
	initOnce                   sync.Once
	metricsMutex               sync.RWMutex
	// Dispatcher-specific metrics
	dispatcherActive          metric.Int64UpDownCounter
	dispatcherHeartbeatTotal  metric.Int64Counter
	dispatcherLifecycleTotal  metric.Int64Counter
	dispatcherTakeoverTotal   metric.Int64Counter
	dispatcherTakeoverLatency metric.Float64Histogram
	dispatcherUptimeSeconds   metric.Float64ObservableGauge
	dispatcherUptimeCallback  metric.Registration
	dispatcherStartTimes      sync.Map // map[string]time.Time for tracking uptime per dispatcher
	// Dispatcher key scan metrics
	dispatcherKeysScannedTotal metric.Int64Counter
	dispatcherStaleFoundTotal  metric.Int64Counter
	dispatcherScanDuration     metric.Float64Histogram
)

var workflowDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// resetMetrics is used for testing purposes only
func resetMetrics(ctx context.Context) {
	if callbackRegistration != nil {
		err := callbackRegistration.Unregister()
		if err != nil {
			log := logger.FromContext(ctx)
			log.Debug("Failed to unregister callback during reset", "error", err, "component", "temporal_metrics")
		}
		callbackRegistration = nil
	}
	if dispatcherUptimeCallback != nil {
		err := dispatcherUptimeCallback.Unregister()
		if err != nil {
			log := logger.FromContext(ctx)
			log.Debug(
				"Failed to unregister dispatcher uptime callback during reset",
				"error",
				err,
				"component",
				"temporal_metrics",
			)
		}
		dispatcherUptimeCallback = nil
	}
	workflowStartedTotal = nil
	workflowCompletedTotal = nil
	workflowFailedTotal = nil
	workflowTaskDuration = nil
	workersRunning = nil
	workersConfigured = nil
	configuredWorkerCountValue.Store(0)
	dispatcherActive = nil
	dispatcherHeartbeatTotal = nil
	dispatcherLifecycleTotal = nil
	dispatcherTakeoverTotal = nil
	dispatcherTakeoverLatency = nil
	dispatcherUptimeSeconds = nil
	dispatcherKeysScannedTotal = nil
	dispatcherStaleFoundTotal = nil
	dispatcherScanDuration = nil
	dispatcherStartTimes = sync.Map{}
	initOnce = sync.Once{}
}

// ResetMetricsForTesting resets the metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetMetricsForTesting(ctx context.Context) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	resetMetrics(ctx)
}

// initWorkflowMetrics initializes workflow-related metrics
func initWorkflowMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	var err error
	workflowStartedTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("temporal", "workflow_started_total"),
		metric.WithDescription("Started workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow started counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowCompletedTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("temporal", "workflow_completed_total"),
		metric.WithDescription("Completed workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow completed counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowFailedTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("temporal", "workflow_failed_total"),
		metric.WithDescription("Failed workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow failed counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowTaskDuration, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("temporal", "workflow_duration_seconds"),
		metric.WithDescription("Workflow execution time"),
		metric.WithExplicitBucketBoundaries(workflowDurationBuckets...),
	)
	if err != nil {
		log.Error("Failed to create workflow task duration histogram", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

// initWorkerMetrics initializes worker-related metrics
func initWorkerMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	var err error
	workersRunning, err = meter.Int64UpDownCounter(
		metrics.MetricNameWithSubsystem("temporal", "workers_running_total"),
		metric.WithDescription("Currently running workers"),
	)
	if err != nil {
		log.Error("Failed to create workers running counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workersConfigured, err = meter.Int64ObservableGauge(
		metrics.MetricNameWithSubsystem("temporal", "workers_configured_total"),
		metric.WithDescription("Configured workers per instance"),
	)
	if err != nil {
		log.Error("Failed to create workers configured gauge", "error", err, "component", "temporal_metrics")
		return err
	}
	callbackRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(workersConfigured, configuredWorkerCountValue.Load())
		return nil
	}, workersConfigured)
	if err != nil {
		log.Error(
			"Failed to register callback for workers configured gauge",
			"error",
			err,
			"component",
			"temporal_metrics",
		)
		return err
	}
	return nil
}

// initDispatcherMetrics initializes dispatcher-related metrics
func initDispatcherMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	if err := createDispatcherCoreMetrics(meter, log); err != nil {
		return err
	}
	if err := initDispatcherTakeoverMetrics(ctx, meter); err != nil {
		return err
	}
	if err := initDispatcherScanMetrics(ctx, meter); err != nil {
		return err
	}
	if err := setupDispatcherUptimeGauge(meter, log); err != nil {
		return err
	}
	return registerDispatcherUptime(meter, log)
}

// createDispatcherCoreMetrics defines primary dispatcher metric instruments.
func createDispatcherCoreMetrics(meter metric.Meter, log logger.Logger) error {
	var err error
	dispatcherActive, err = meter.Int64UpDownCounter(
		metrics.MetricNameWithSubsystem("dispatcher", "active_total"),
		metric.WithDescription("Currently active dispatchers"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher active counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherHeartbeatTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("dispatcher", "heartbeat_total"),
		metric.WithDescription("Total dispatcher heartbeats"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher heartbeat counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherLifecycleTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("dispatcher", "lifecycle_events_total"),
		metric.WithDescription("Total dispatcher lifecycle events"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher lifecycle counter", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

// setupDispatcherUptimeGauge creates the observable gauge for dispatcher uptime.
func setupDispatcherUptimeGauge(meter metric.Meter, log logger.Logger) error {
	var err error
	dispatcherUptimeSeconds, err = meter.Float64ObservableGauge(
		metrics.MetricNameWithSubsystem("dispatcher", "uptime_seconds"),
		metric.WithDescription("Dispatcher uptime in seconds"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher uptime gauge", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

// registerDispatcherUptime registers the callback responsible for uptime observations.
func registerDispatcherUptime(meter metric.Meter, log logger.Logger) error {
	callback, err := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		reportDispatcherUptime(o)
		return nil
	}, dispatcherUptimeSeconds)
	if err != nil {
		log.Error("Failed to register dispatcher uptime callback", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherUptimeCallback = callback
	return nil
}

// reportDispatcherUptime reports dispatcher uptime observations.
func reportDispatcherUptime(observer metric.Observer) {
	now := time.Now()
	dispatcherStartTimes.Range(func(key, value any) bool {
		dispatcherID, ok := key.(string)
		if !ok {
			return true
		}
		startTime, ok := value.(time.Time)
		if !ok {
			return true
		}
		uptime := now.Sub(startTime).Seconds()
		observer.ObserveFloat64(
			dispatcherUptimeSeconds,
			uptime,
			metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)),
		)
		return true
	})
}

func initDispatcherTakeoverMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	var err error
	dispatcherTakeoverTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("dispatcher", "takeover_total"),
		metric.WithDescription("Total dispatcher takeover attempts"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher takeover counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherTakeoverLatency, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("dispatcher", "takeover_latency_seconds"),
		metric.WithDescription("Dispatcher takeover latency in seconds"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	)
	if err != nil {
		log.Error(
			"Failed to create dispatcher takeover latency histogram",
			"error",
			err,
			"component",
			"temporal_metrics",
		)
		return err
	}
	return nil
}

// initDispatcherScanMetrics initializes metrics related to dispatcher key scans.
func initDispatcherScanMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	var err error
	dispatcherKeysScannedTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("dispatcher", "keys_scanned_total"),
		metric.WithDescription("Total dispatcher heartbeat keys scanned"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher keys scanned counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherStaleFoundTotal, err = meter.Int64Counter(
		metrics.MetricNameWithSubsystem("dispatcher", "stale_heartbeats_total"),
		metric.WithDescription("Total stale dispatcher heartbeats encountered during scans"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher stale counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherScanDuration, err = meter.Float64Histogram(
		metrics.MetricNameWithSubsystem("dispatcher", "scan_duration_seconds"),
		metric.WithDescription("Duration of dispatcher heartbeat scans"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5),
	)
	if err != nil {
		log.Error("Failed to create dispatcher scan duration histogram", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

func initMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	initOnce.Do(func() {
		if err := initWorkflowMetrics(ctx, meter); err != nil {
			return
		}
		if err := initWorkerMetrics(ctx, meter); err != nil {
			return
		}
		if err := initDispatcherMetrics(ctx, meter); err != nil {
			return
		}
	})
}

// TemporalMetrics creates a new Temporal metrics interceptor
func TemporalMetrics(ctx context.Context, meter metric.Meter) interceptor.WorkerInterceptor {
	if meter == nil {
		// NOTE: Return a no-op interceptor when metrics collection is disabled to keep workers running.
		log := logger.FromContext(ctx)
		log.Warn("TemporalMetrics called with nil meter, returning no-op interceptor")
		return &interceptor.WorkerInterceptorBase{}
	}
	initMetrics(ctx, meter)
	return &metricsInterceptor{
		meter:   meter,
		baseCtx: context.WithoutCancel(ctx),
	}
}

type metricsInterceptor struct {
	interceptor.WorkerInterceptorBase
	meter   metric.Meter
	baseCtx context.Context
}

// InterceptWorkflow intercepts workflow execution for metrics collection
func (m *metricsInterceptor) InterceptWorkflow(
	_ workflow.Context,
	next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return &workflowInboundInterceptor{
		WorkflowInboundInterceptorBase: interceptor.WorkflowInboundInterceptorBase{
			Next: next,
		},
		meter:   m.meter,
		baseCtx: m.baseCtx,
	}
}

type workflowInboundInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	meter   metric.Meter
	baseCtx context.Context
}

// workflowMetricSet groups the instrumentation required for workflow observations.
type workflowMetricSet struct {
	started   metric.Int64Counter
	duration  metric.Float64Histogram
	failed    metric.Int64Counter
	completed metric.Int64Counter
}

// collectWorkflowMetrics safely retrieves the workflow metric instruments.
func collectWorkflowMetrics() (workflowMetricSet, bool) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	missingMetrics := workflowStartedTotal == nil ||
		workflowTaskDuration == nil ||
		workflowFailedTotal == nil ||
		workflowCompletedTotal == nil
	if missingMetrics {
		return workflowMetricSet{}, false
	}
	return workflowMetricSet{
		started:   workflowStartedTotal,
		duration:  workflowTaskDuration,
		failed:    workflowFailedTotal,
		completed: workflowCompletedTotal,
	}, true
}

// ExecuteWorkflow records metrics for workflow execution
func (w *workflowInboundInterceptor) ExecuteWorkflow(
	ctx workflow.Context,
	in *interceptor.ExecuteWorkflowInput,
) (any, error) {
	metrics, ok := collectWorkflowMetrics()
	if !ok || workflow.IsReplaying(ctx) {
		return w.Next.ExecuteWorkflow(ctx, in)
	}
	defer w.recoverFromWorkflowPanic()
	startTime := workflow.Now(ctx)
	info := workflow.GetInfo(ctx)
	workflowType := info.WorkflowType.Name
	otelCtx := w.baseCtx
	metrics.started.Add(otelCtx, 1, metric.WithAttributes(attribute.String("workflow_type", workflowType)))
	result, err := w.Next.ExecuteWorkflow(ctx, in)
	duration := workflow.Now(ctx).Sub(startTime).Seconds()
	w.recordWorkflowOutcome(otelCtx, metrics, duration, workflowType, info, err)
	return result, err
}

// recoverFromWorkflowPanic logs and rethrows panics during workflow interception.
func (w *workflowInboundInterceptor) recoverFromWorkflowPanic() {
	if r := recover(); r != nil {
		logger.FromContext(w.baseCtx).Error("Panic in Temporal metrics interceptor", "panic", r)
		panic(r)
	}
}

// recordWorkflowOutcome records completion or failure metrics and logs diagnostics.
func (w *workflowInboundInterceptor) recordWorkflowOutcome(
	ctx context.Context,
	metrics workflowMetricSet,
	duration float64,
	workflowType string,
	info *workflow.Info,
	err error,
) {
	if err != nil {
		label, message := classifyWorkflowError(err)
		metrics.duration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("workflow_type", workflowType),
				attribute.String("result", label),
			))
		metrics.failed.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("workflow_type", workflowType),
				attribute.String("result", label),
			))
		logger.FromContext(w.baseCtx).Debug(
			message,
			"workflow_type", workflowType,
			"workflow_id", info.WorkflowExecution.ID,
			"error", err,
		)
		return
	}
	metrics.duration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("workflow_type", workflowType),
			attribute.String("result", "completed"),
		))
	metrics.completed.Add(ctx, 1,
		metric.WithAttributes(attribute.String("workflow_type", workflowType)))
}

// classifyWorkflowError maps workflow errors to result labels and log messages.
func classifyWorkflowError(err error) (string, string) {
	switch {
	case temporal.IsCanceledError(err) || err == workflow.ErrCanceled:
		return "canceled", "Workflow canceled"
	case temporal.IsTimeoutError(err):
		return "timeout", "Workflow timed out"
	default:
		return "failed", "Workflow failed"
	}
}

// SetConfiguredWorkerCount sets the configured worker count gauge
func SetConfiguredWorkerCount(count int64) {
	configuredWorkerCountValue.Store(count)
}

func metricsContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

// IncrementRunningWorkers increments the running workers counter
func IncrementRunningWorkers(ctx context.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if workersRunning != nil {
		workersRunning.Add(metricsContext(ctx), 1)
	}
}

// DecrementRunningWorkers decrements the running workers counter
func DecrementRunningWorkers(ctx context.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if workersRunning != nil {
		workersRunning.Add(metricsContext(ctx), -1)
	}
}

// StartDispatcher records dispatcher start event and tracks uptime
func StartDispatcher(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherActive != nil {
		dispatcherActive.Add(
			metricsContext(ctx),
			1,
			metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)),
		)
	}
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(metricsContext(ctx), 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "start")))
	}
	dispatcherStartTimes.Store(dispatcherID, time.Now())
}

// StopDispatcher records dispatcher stop event and removes uptime tracking
func StopDispatcher(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherActive != nil {
		dispatcherActive.Add(
			metricsContext(ctx),
			-1,
			metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)),
		)
	}
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(metricsContext(ctx), 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "stop")))
	}
	dispatcherStartTimes.Delete(dispatcherID)
}

// RecordDispatcherHeartbeat records a dispatcher heartbeat event
func RecordDispatcherHeartbeat(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherHeartbeatTotal != nil {
		dispatcherHeartbeatTotal.Add(
			metricsContext(ctx),
			1,
			metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)),
		)
	}
}

// RecordDispatcherScan records metrics for a dispatcher heartbeat scan operation.
func RecordDispatcherScan(ctx context.Context, keysScanned int64, staleFound int64, duration time.Duration) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherKeysScannedTotal != nil {
		dispatcherKeysScannedTotal.Add(metricsContext(ctx), keysScanned)
	}
	if dispatcherStaleFoundTotal != nil && staleFound > 0 {
		dispatcherStaleFoundTotal.Add(metricsContext(ctx), staleFound)
	}
	if dispatcherScanDuration != nil {
		dispatcherScanDuration.Record(metricsContext(ctx), duration.Seconds())
	}
}

// RecordDispatcherRestart records dispatcher restart event
func RecordDispatcherRestart(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(metricsContext(ctx), 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "restart")))
	}
	dispatcherStartTimes.Store(dispatcherID, time.Now())
}

func RecordDispatcherTakeover(ctx context.Context, dispatcherID string, duration time.Duration, outcome string) {
	metricsMutex.RLock()
	takeoverTotal := dispatcherTakeoverTotal
	takeoverLatency := dispatcherTakeoverLatency
	metricsMutex.RUnlock()
	attrs := []attribute.KeyValue{attribute.String("dispatcher_id", dispatcherID)}
	if outcome != "" {
		attrs = append(attrs, attribute.String("outcome", outcome))
	}
	if takeoverTotal != nil {
		takeoverTotal.Add(metricsContext(ctx), 1, metric.WithAttributes(attrs...))
	}
	if takeoverLatency != nil {
		takeoverLatency.Record(metricsContext(ctx), duration.Seconds(), metric.WithAttributes(attrs...))
	}
}
