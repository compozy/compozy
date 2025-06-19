package interceptor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

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
	dispatcherActive         metric.Int64UpDownCounter
	dispatcherHeartbeatTotal metric.Int64Counter
	dispatcherLifecycleTotal metric.Int64Counter
	dispatcherUptimeSeconds  metric.Float64ObservableGauge
	dispatcherUptimeCallback metric.Registration
	dispatcherStartTimes     sync.Map // map[string]time.Time for tracking uptime per dispatcher
)

// resetMetrics is used for testing purposes only
func resetMetrics(ctx context.Context) {
	// Unregister callbacks if they exist
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
	// Reset dispatcher metrics
	dispatcherActive = nil
	dispatcherHeartbeatTotal = nil
	dispatcherLifecycleTotal = nil
	dispatcherUptimeSeconds = nil
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
func initWorkflowMetrics(meter metric.Meter, log logger.Logger) error {
	var err error
	workflowStartedTotal, err = meter.Int64Counter(
		"compozy_temporal_workflow_started_total",
		metric.WithDescription("Started workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow started counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowCompletedTotal, err = meter.Int64Counter(
		"compozy_temporal_workflow_completed_total",
		metric.WithDescription("Completed workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow completed counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowFailedTotal, err = meter.Int64Counter(
		"compozy_temporal_workflow_failed_total",
		metric.WithDescription("Failed workflows"),
	)
	if err != nil {
		log.Error("Failed to create workflow failed counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workflowTaskDuration, err = meter.Float64Histogram(
		"compozy_temporal_workflow_duration_seconds",
		metric.WithDescription("Workflow execution time"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	)
	if err != nil {
		log.Error("Failed to create workflow task duration histogram", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

// initWorkerMetrics initializes worker-related metrics
func initWorkerMetrics(meter metric.Meter, log logger.Logger) error {
	var err error
	workersRunning, err = meter.Int64UpDownCounter(
		"compozy_temporal_workers_running_total",
		metric.WithDescription("Currently running workers"),
	)
	if err != nil {
		log.Error("Failed to create workers running counter", "error", err, "component", "temporal_metrics")
		return err
	}
	workersConfigured, err = meter.Int64ObservableGauge(
		"compozy_temporal_workers_configured_total",
		metric.WithDescription("Configured workers per instance"),
	)
	if err != nil {
		log.Error("Failed to create workers configured gauge", "error", err, "component", "temporal_metrics")
		return err
	}
	// Register the callback only once during initialization
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
func initDispatcherMetrics(meter metric.Meter, log logger.Logger) error {
	var err error
	dispatcherActive, err = meter.Int64UpDownCounter(
		"compozy_dispatcher_active_total",
		metric.WithDescription("Currently active dispatchers"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher active counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherHeartbeatTotal, err = meter.Int64Counter(
		"compozy_dispatcher_heartbeat_total",
		metric.WithDescription("Total dispatcher heartbeats"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher heartbeat counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherLifecycleTotal, err = meter.Int64Counter(
		"compozy_dispatcher_lifecycle_events_total",
		metric.WithDescription("Total dispatcher lifecycle events"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher lifecycle counter", "error", err, "component", "temporal_metrics")
		return err
	}
	dispatcherUptimeSeconds, err = meter.Float64ObservableGauge(
		"compozy_dispatcher_uptime_seconds",
		metric.WithDescription("Dispatcher uptime in seconds"),
	)
	if err != nil {
		log.Error("Failed to create dispatcher uptime gauge", "error", err, "component", "temporal_metrics")
		return err
	}
	// Register dispatcher uptime callback
	dispatcherUptimeCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		now := time.Now()
		dispatcherStartTimes.Range(func(key, value any) bool {
			dispatcherID, ok := key.(string)
			if !ok {
				return true // Skip invalid key
			}
			startTime, ok := value.(time.Time)
			if !ok {
				return true // Skip invalid value
			}
			uptime := now.Sub(startTime).Seconds()
			o.ObserveFloat64(dispatcherUptimeSeconds, uptime,
				metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)))
			return true
		})
		return nil
	}, dispatcherUptimeSeconds)
	if err != nil {
		log.Error("Failed to register dispatcher uptime callback", "error", err, "component", "temporal_metrics")
		return err
	}
	return nil
}

func initMetrics(ctx context.Context, meter metric.Meter) {
	// Skip initialization if meter is nil
	if meter == nil {
		return
	}
	log := logger.FromContext(ctx)
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	initOnce.Do(func() {
		if err := initWorkflowMetrics(meter, log); err != nil {
			return
		}
		if err := initWorkerMetrics(meter, log); err != nil {
			return
		}
		if err := initDispatcherMetrics(meter, log); err != nil {
			return
		}
	})
}

// TemporalMetrics creates a new Temporal metrics interceptor
func TemporalMetrics(ctx context.Context, meter metric.Meter) interceptor.WorkerInterceptor {
	// Handle nil meter gracefully
	if meter == nil {
		log := logger.FromContext(ctx)
		log.Warn("TemporalMetrics called with nil meter, returning no-op interceptor")
		return &interceptor.WorkerInterceptorBase{}
	}
	initMetrics(ctx, meter)
	return &metricsInterceptor{
		meter: meter,
	}
}

type metricsInterceptor struct {
	interceptor.WorkerInterceptorBase
	meter metric.Meter
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
		meter: m.meter,
	}
}

type workflowInboundInterceptor struct {
	interceptor.WorkflowInboundInterceptorBase
	meter metric.Meter
}

// ExecuteWorkflow records metrics for workflow execution
func (w *workflowInboundInterceptor) ExecuteWorkflow(
	ctx workflow.Context,
	in *interceptor.ExecuteWorkflowInput,
) (any, error) {
	// Get local references to metrics instruments under a single lock
	metricsMutex.RLock()
	wst := workflowStartedTotal
	wtd := workflowTaskDuration
	wft := workflowFailedTotal
	wct := workflowCompletedTotal
	metricsMutex.RUnlock()
	// If not initialized, proceed without metrics
	if wst == nil {
		return w.Next.ExecuteWorkflow(ctx, in)
	}
	// Skip metrics during replay to avoid inflating counts
	if workflow.IsReplaying(ctx) {
		return w.Next.ExecuteWorkflow(ctx, in)
	}
	defer func() {
		if r := recover(); r != nil {
			// Use background context for panic logging since workflow context might be corrupted
			log := logger.FromContext(context.Background())
			log.Error("Panic in Temporal metrics interceptor", "panic", r)
			// Re-panic to let Temporal handle it properly
			panic(r)
		}
	}()
	startTime := workflow.Now(ctx) // Use deterministic time
	info := workflow.GetInfo(ctx)
	workflowType := info.WorkflowType.Name
	otelCtx := context.Background()
	wst.Add(otelCtx, 1,
		metric.WithAttributes(attribute.String("workflow_type", workflowType)))
	result, err := w.Next.ExecuteWorkflow(ctx, in)
	// Calculate duration using deterministic time
	duration := workflow.Now(ctx).Sub(startTime).Seconds()
	if err != nil {
		// Distinguish between different error types for better observability
		var resultLabel string
		var logMessage string
		switch {
		case temporal.IsCanceledError(err) || err == workflow.ErrCanceled:
			resultLabel = "canceled"
			logMessage = "Workflow canceled"
		case temporal.IsTimeoutError(err):
			resultLabel = "timeout"
			logMessage = "Workflow timed out"
		default:
			resultLabel = "failed"
			logMessage = "Workflow failed"
		}
		wtd.Record(otelCtx, duration,
			metric.WithAttributes(
				attribute.String("workflow_type", workflowType),
				attribute.String("result", resultLabel)))
		wft.Add(otelCtx, 1,
			metric.WithAttributes(
				attribute.String("workflow_type", workflowType),
				attribute.String("result", resultLabel)))
		// Use background context for logging since workflow context is for Temporal operations
		log := logger.FromContext(context.Background())
		log.Debug(logMessage, "workflow_type", workflowType, "workflow_id", info.WorkflowExecution.ID, "error", err)
	} else {
		wtd.Record(otelCtx, duration,
			metric.WithAttributes(
				attribute.String("workflow_type", workflowType),
				attribute.String("result", "completed")))
		wct.Add(otelCtx, 1,
			metric.WithAttributes(attribute.String("workflow_type", workflowType)))
	}
	return result, err
}

// SetConfiguredWorkerCount sets the configured worker count gauge
func SetConfiguredWorkerCount(count int64) {
	configuredWorkerCountValue.Store(count)
}

// IncrementRunningWorkers increments the running workers counter
func IncrementRunningWorkers(ctx context.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if workersRunning != nil {
		workersRunning.Add(ctx, 1)
	}
}

// DecrementRunningWorkers decrements the running workers counter
func DecrementRunningWorkers(ctx context.Context) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if workersRunning != nil {
		workersRunning.Add(ctx, -1)
	}
}

// StartDispatcher records dispatcher start event and tracks uptime
func StartDispatcher(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherActive != nil {
		dispatcherActive.Add(ctx, 1, metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)))
	}
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "start")))
	}
	// Track start time for uptime calculation
	dispatcherStartTimes.Store(dispatcherID, time.Now())
}

// StopDispatcher records dispatcher stop event and removes uptime tracking
func StopDispatcher(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherActive != nil {
		dispatcherActive.Add(ctx, -1, metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)))
	}
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "stop")))
	}
	// Remove start time tracking
	dispatcherStartTimes.Delete(dispatcherID)
}

// RecordDispatcherHeartbeat records a dispatcher heartbeat event
func RecordDispatcherHeartbeat(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherHeartbeatTotal != nil {
		dispatcherHeartbeatTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("dispatcher_id", dispatcherID)))
	}
}

// RecordDispatcherRestart records dispatcher restart event
func RecordDispatcherRestart(ctx context.Context, dispatcherID string) {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	if dispatcherLifecycleTotal != nil {
		dispatcherLifecycleTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("dispatcher_id", dispatcherID),
				attribute.String("event", "restart")))
	}
	// Update start time for uptime calculation
	dispatcherStartTimes.Store(dispatcherID, time.Now())
}
