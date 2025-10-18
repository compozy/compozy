package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/sdk/activity"
	temporalclient "go.temporal.io/sdk/client"
	temporalinterceptor "go.temporal.io/sdk/interceptor"
	temporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	workerMetricSubsystem  = "worker"
	queueDepthSamplePeriod = 15 * time.Second
	activityLabelUnknown   = "unknown"
	queueLabelUnknown      = "unknown"
)

var activityDurationBuckets = []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300}

var (
	metricsInitOnce sync.Once
	initMetricsErr  error

	activitiesExecutingGauge  metric.Int64ObservableGauge
	workflowsExecutingGauge   metric.Int64ObservableGauge
	taskQueueDepthGauge       metric.Int64ObservableGauge
	workerUtilizationGauge    metric.Float64ObservableGauge
	activityDurationHistogram metric.Float64Histogram

	metricsCallback metric.Registration

	activitiesExecutingCount atomic.Int64
	workflowsExecutingCount  atomic.Int64
	maxConcurrentActivities  atomic.Int64
	maxConcurrentWorkflows   atomic.Int64

	queueDepthEntries sync.Map
)

type queueDepthEntry struct {
	value atomic.Int64
}

func ensureWorkerMetrics() {
	metricsInitOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("compozy.worker")
		if err := initWorkerInstruments(meter); err != nil {
			initMetricsErr = err
			return
		}
	})
	if initMetricsErr != nil {
		panic(initMetricsErr)
	}
}

func initWorkerInstruments(meter metric.Meter) error {
	if metricsCallback != nil {
		if err := metricsCallback.Unregister(); err != nil {
			return fmt.Errorf("worker metrics: unregister callback: %w", err)
		}
	}
	if err := initWorkerGauges(meter); err != nil {
		return err
	}
	if err := initWorkerHistograms(meter); err != nil {
		return err
	}
	callbackInstruments := []metric.Observable{
		activitiesExecutingGauge,
		workflowsExecutingGauge,
		taskQueueDepthGauge,
		workerUtilizationGauge,
	}
	registration, err := meter.RegisterCallback(observeWorkerMetrics, callbackInstruments...)
	if err != nil {
		return fmt.Errorf("worker metrics: register callback: %w", err)
	}
	metricsCallback = registration
	return nil
}

func initWorkerGauges(meter metric.Meter) error {
	var err error
	activitiesExecutingGauge, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(workerMetricSubsystem, "activities_executing"),
		metric.WithDescription("Number of activities currently executing"),
	)
	if err != nil {
		return fmt.Errorf("worker metrics: activities gauge: %w", err)
	}
	workflowsExecutingGauge, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(workerMetricSubsystem, "workflows_executing"),
		metric.WithDescription("Number of workflows currently executing"),
	)
	if err != nil {
		return fmt.Errorf("worker metrics: workflows gauge: %w", err)
	}
	taskQueueDepthGauge, err = meter.Int64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(workerMetricSubsystem, "task_queue_depth"),
		metric.WithDescription("Number of tasks waiting in queue"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("worker metrics: queue depth gauge: %w", err)
	}
	workerUtilizationGauge, err = meter.Float64ObservableGauge(
		monitoringmetrics.MetricNameWithSubsystem(workerMetricSubsystem, "utilization_ratio"),
		metric.WithDescription("Worker utilization (executing / max_concurrent)"),
	)
	if err != nil {
		return fmt.Errorf("worker metrics: utilization gauge: %w", err)
	}
	return nil
}

func initWorkerHistograms(meter metric.Meter) error {
	var err error
	activityDurationHistogram, err = meter.Float64Histogram(
		monitoringmetrics.MetricNameWithSubsystem(workerMetricSubsystem, "activity_duration_seconds"),
		metric.WithDescription("Activity execution duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(activityDurationBuckets...),
	)
	if err != nil {
		return fmt.Errorf("worker metrics: activity duration histogram: %w", err)
	}
	return nil
}

func observeWorkerMetrics(_ context.Context, observer metric.Observer) error {
	observer.ObserveInt64(activitiesExecutingGauge, clampNonNegative(activitiesExecutingCount.Load()))
	observer.ObserveInt64(workflowsExecutingGauge, clampNonNegative(workflowsExecutingCount.Load()))
	queueDepthEntries.Range(func(key, value any) bool {
		queueName, ok := key.(string)
		if !ok || queueName == "" {
			return true
		}
		entry, ok := value.(*queueDepthEntry)
		if !ok || entry == nil {
			return true
		}
		depth := clampNonNegative(entry.value.Load())
		observer.ObserveInt64(
			taskQueueDepthGauge,
			depth,
			metric.WithAttributes(attribute.String("queue_name", queueName)),
		)
		return true
	})
	recordUtilization(observer, "activity", activitiesExecutingCount.Load(), maxConcurrentActivities.Load())
	recordUtilization(observer, "workflow", workflowsExecutingCount.Load(), maxConcurrentWorkflows.Load())
	return nil
}

func recordUtilization(observer metric.Observer, workerType string, executing int64, maxCapacity int64) {
	if workerUtilizationGauge == nil {
		return
	}
	var ratio float64
	if maxCapacity > 0 && executing > 0 {
		ratio = float64(executing) / float64(maxCapacity)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
	}
	observer.ObserveFloat64(
		workerUtilizationGauge,
		ratio,
		metric.WithAttributes(attribute.String("worker_type", workerType)),
	)
}

func clampNonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func metricsContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func normalizeActivityType(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return activityLabelUnknown
	}
	return clean
}

func classifyActivityOutcome(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, activity.ErrResultPending):
		return "success"
	case temporal.IsTimeoutError(err):
		return "timeout"
	default:
		return "error"
	}
}

func recordActivityDuration(ctx context.Context, activityType string, outcome string, duration time.Duration) {
	if activityDurationHistogram == nil {
		return
	}
	activityDurationHistogram.Record(
		metricsContext(ctx),
		duration.Seconds(),
		metric.WithAttributes(
			attribute.String("activity_type", normalizeActivityType(activityType)),
			attribute.String("outcome", outcome),
		),
	)
}

func incrementActivitiesExecuting() {
	activitiesExecutingCount.Add(1)
}

func decrementActivitiesExecuting() {
	if activitiesExecutingCount.Add(-1) < 0 {
		activitiesExecutingCount.Store(0)
	}
}

func incrementWorkflowsExecuting() {
	workflowsExecutingCount.Add(1)
}

func decrementWorkflowsExecuting() {
	if workflowsExecutingCount.Add(-1) < 0 {
		workflowsExecutingCount.Store(0)
	}
}

func recordTaskQueueDepth(queue string, depth int64) {
	if queue == "" {
		queue = queueLabelUnknown
	}
	value := clampNonNegative(depth)
	entryAny, _ := queueDepthEntries.LoadOrStore(queue, &queueDepthEntry{})
	entry, ok := entryAny.(*queueDepthEntry)
	if !ok {
		entry = &queueDepthEntry{}
		queueDepthEntries.Store(queue, entry)
	}
	entry.value.Store(value)
}

func setWorkerConcurrencyLimits(activityLimit int, workflowLimit int) {
	if activityLimit < 0 {
		activityLimit = 0
	}
	if workflowLimit < 0 {
		workflowLimit = 0
	}
	maxConcurrentActivities.Store(int64(activityLimit))
	maxConcurrentWorkflows.Store(int64(workflowLimit))
}

func newWorkerMetricsInterceptor(ctx context.Context) temporalinterceptor.WorkerInterceptor {
	ensureWorkerMetrics()
	return &workerMetricsInterceptor{
		baseCtx: metricsContext(ctx),
	}
}

type workerMetricsInterceptor struct {
	temporalinterceptor.WorkerInterceptorBase
	baseCtx context.Context
}

func (i *workerMetricsInterceptor) InterceptWorkflow(
	_ workflow.Context,
	next temporalinterceptor.WorkflowInboundInterceptor,
) temporalinterceptor.WorkflowInboundInterceptor {
	ensureWorkerMetrics()
	return &workflowMetricsInbound{
		WorkflowInboundInterceptorBase: temporalinterceptor.WorkflowInboundInterceptorBase{
			Next: next,
		},
	}
}

func (i *workerMetricsInterceptor) InterceptActivity(
	ctx context.Context,
	next temporalinterceptor.ActivityInboundInterceptor,
) temporalinterceptor.ActivityInboundInterceptor {
	ensureWorkerMetrics()
	return &activityMetricsInbound{
		ActivityInboundInterceptorBase: temporalinterceptor.ActivityInboundInterceptorBase{
			Next: next,
		},
		baseCtx: metricsContext(ctx),
	}
}

type workflowMetricsInbound struct {
	temporalinterceptor.WorkflowInboundInterceptorBase
}

func (w *workflowMetricsInbound) ExecuteWorkflow(
	ctx workflow.Context,
	in *temporalinterceptor.ExecuteWorkflowInput,
) (result any, err error) {
	incrementWorkflowsExecuting()
	defer func() {
		defer decrementWorkflowsExecuting()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	return w.Next.ExecuteWorkflow(ctx, in)
}

type activityMetricsInbound struct {
	temporalinterceptor.ActivityInboundInterceptorBase
	baseCtx context.Context
}

func (a *activityMetricsInbound) ExecuteActivity(
	ctx context.Context,
	in *temporalinterceptor.ExecuteActivityInput,
) (result any, err error) {
	incrementActivitiesExecuting()
	start := time.Now()
	actCtx := metricsContext(ctx)
	info := activity.GetInfo(ctx)
	activityType := activityLabelUnknown
	if name := info.ActivityType.Name; strings.TrimSpace(name) != "" {
		activityType = normalizeActivityType(name)
	}
	defer func() {
		duration := time.Since(start)
		outcome := classifyActivityOutcome(err)
		if r := recover(); r != nil {
			recordActivityDuration(actCtx, activityType, "error", duration)
			decrementActivitiesExecuting()
			panic(r)
		}
		recordActivityDuration(actCtx, activityType, outcome, duration)
		decrementActivitiesExecuting()
	}()
	return a.Next.ExecuteActivity(ctx, in)
}

func startTaskQueueDepthSampler(ctx context.Context, client *Client, queue string) context.CancelFunc {
	if client == nil || strings.TrimSpace(queue) == "" {
		return nil
	}
	ensureWorkerMetrics()
	recordTaskQueueDepth(queue, 0)
	monitorCtx, cancel := context.WithCancel(metricsContext(ctx))
	go runTaskQueueSampler(monitorCtx, client, queue)
	return cancel
}

func runTaskQueueSampler(ctx context.Context, client *Client, queue string) {
	log := logger.FromContext(ctx)
	ticker := time.NewTicker(queueDepthSamplePeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			recordTaskQueueDepth(queue, 0)
			return
		default:
		}
		depth, err := fetchTaskQueueDepth(ctx, client, queue)
		if err != nil {
			log.Debug("Failed to sample task queue depth", "error", err, "task_queue", queue)
		} else {
			recordTaskQueueDepth(queue, depth)
		}
		select {
		case <-ctx.Done():
			recordTaskQueueDepth(queue, 0)
			return
		case <-ticker.C:
		}
	}
}

func fetchTaskQueueDepth(ctx context.Context, client *Client, queue string) (int64, error) {
	desc, err := client.DescribeTaskQueueEnhanced(ctx, temporalclient.DescribeTaskQueueEnhancedOptions{
		TaskQueue: queue,
		TaskQueueTypes: []temporalclient.TaskQueueType{
			temporalclient.TaskQueueTypeWorkflow,
			temporalclient.TaskQueueTypeActivity,
		},
		ReportStats: true,
	})
	if err != nil {
		return 0, err
	}
	var total int64
	for _, versionInfo := range desc.VersionsInfo {
		for _, typeInfo := range versionInfo.TypesInfo {
			if typeInfo.Stats != nil {
				total += typeInfo.Stats.ApproximateBacklogCount
			}
		}
	}
	return total, nil
}

func (w *Worker) startQueueDepthMonitor() {
	if w == nil || w.queueDepthCancel != nil {
		return
	}
	ctx := w.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	w.queueDepthCancel = startTaskQueueDepthSampler(ctx, w.client, w.taskQueue)
}

func (w *Worker) stopQueueDepthMonitor() {
	if w == nil {
		return
	}
	if w.queueDepthCancel != nil {
		w.queueDepthCancel()
		w.queueDepthCancel = nil
	}
	if strings.TrimSpace(w.taskQueue) != "" {
		recordTaskQueueDepth(w.taskQueue, 0)
	}
}
