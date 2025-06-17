package interceptor

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/sdk/interceptor"
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
	resetMutex                 sync.Mutex
)

// resetMetrics is used for testing purposes only
func resetMetrics() {
	// Unregister callback if it exists
	if callbackRegistration != nil {
		err := callbackRegistration.Unregister()
		if err != nil {
			logger.Error("Failed to unregister callback during reset", "error", err)
		}
		callbackRegistration = nil
	}
	workflowStartedTotal = nil
	workflowCompletedTotal = nil
	workflowFailedTotal = nil
	workflowTaskDuration = nil
	workersRunning = nil
	workersConfigured = nil
	configuredWorkerCountValue.Store(0)
	initOnce = sync.Once{}
}

// ResetMetricsForTesting resets the metrics initialization state for testing
// This should only be used in tests to ensure clean state between test runs
func ResetMetricsForTesting() {
	resetMutex.Lock()
	defer resetMutex.Unlock()
	resetMetrics()
}

func initMetrics(meter metric.Meter) {
	// Skip initialization if meter is nil
	if meter == nil {
		return
	}
	initOnce.Do(func() {
		var err error
		workflowStartedTotal, err = meter.Int64Counter(
			"compozy_temporal_workflow_started_total",
			metric.WithDescription("Started workflows"),
		)
		if err != nil {
			logger.Error("Failed to create workflow started counter", "error", err)
		}
		workflowCompletedTotal, err = meter.Int64Counter(
			"compozy_temporal_workflow_completed_total",
			metric.WithDescription("Completed workflows"),
		)
		if err != nil {
			logger.Error("Failed to create workflow completed counter", "error", err)
		}
		workflowFailedTotal, err = meter.Int64Counter(
			"compozy_temporal_workflow_failed_total",
			metric.WithDescription("Failed workflows"),
		)
		if err != nil {
			logger.Error("Failed to create workflow failed counter", "error", err)
		}
		workflowTaskDuration, err = meter.Float64Histogram(
			"compozy_temporal_workflow_duration_seconds",
			metric.WithDescription("Workflow execution time"),
			metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
		)
		if err != nil {
			logger.Error("Failed to create workflow task duration histogram", "error", err)
		}
		workersRunning, err = meter.Int64UpDownCounter(
			"compozy_temporal_workers_running_total",
			metric.WithDescription("Currently running workers"),
		)
		if err != nil {
			logger.Error("Failed to create workers running counter", "error", err)
		}
		workersConfigured, err = meter.Int64ObservableGauge(
			"compozy_temporal_workers_configured_total",
			metric.WithDescription("Configured workers per instance"),
		)
		if err != nil {
			logger.Error("Failed to create workers configured gauge", "error", err)
			return
		}
		// Register the callback only once during initialization
		callbackRegistration, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			o.ObserveInt64(workersConfigured, configuredWorkerCountValue.Load())
			return nil
		}, workersConfigured)
		if err != nil {
			logger.Error("Failed to register callback for workers configured gauge", "error", err)
		}
	})
}

// TemporalMetrics creates a new Temporal metrics interceptor
func TemporalMetrics(meter metric.Meter) interceptor.WorkerInterceptor {
	// Handle nil meter gracefully
	if meter == nil {
		logger.Error("TemporalMetrics called with nil meter, metrics will not be recorded")
		return &metricsInterceptor{meter: nil}
	}
	initMetrics(meter)
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
	// Check if metrics have been initialized
	if workflowStartedTotal == nil {
		return w.Next.ExecuteWorkflow(ctx, in)
	}
	// Skip metrics during replay to avoid inflating counts
	if workflow.IsReplaying(ctx) {
		return w.Next.ExecuteWorkflow(ctx, in)
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in Temporal metrics interceptor", "panic", r)
			// Re-panic to let Temporal handle it properly
			panic(r)
		}
	}()
	startTime := workflow.Now(ctx) // Use deterministic time
	info := workflow.GetInfo(ctx)
	workflowType := info.WorkflowType.Name
	otelCtx := context.Background()
	workflowStartedTotal.Add(otelCtx, 1,
		metric.WithAttributes(attribute.String("workflow_type", workflowType)))
	result, err := w.Next.ExecuteWorkflow(ctx, in)
	// Calculate duration using deterministic time
	duration := workflow.Now(ctx).Sub(startTime).Seconds()
	workflowTaskDuration.Record(otelCtx, duration,
		metric.WithAttributes(attribute.String("workflow_type", workflowType)))
	if err != nil {
		// All errors are counted as failures
		workflowFailedTotal.Add(otelCtx, 1,
			metric.WithAttributes(attribute.String("workflow_type", workflowType)))
		logger.Debug("Workflow failed", "workflow_type", workflowType, "error", err)
	} else {
		workflowCompletedTotal.Add(otelCtx, 1,
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
	if workersRunning != nil {
		workersRunning.Add(ctx, 1)
	}
}

// DecrementRunningWorkers decrements the running workers counter
func DecrementRunningWorkers(ctx context.Context) {
	if workersRunning != nil {
		workersRunning.Add(ctx, -1)
	}
}
