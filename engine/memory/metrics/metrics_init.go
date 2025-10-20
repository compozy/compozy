package metrics

import (
	"context"
	"sync"

	monitoringmetrics "github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// InitMemoryMetrics initializes memory system metrics
func InitMemoryMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	memoryMetricsOnce.Do(func() {
		performMetricsInitialization(ctx, meter)
	})
}

func performMetricsInitialization(ctx context.Context, meter metric.Meter) {
	log := logger.FromContext(ctx)
	initializers := []func(context.Context, metric.Meter) error{
		initCounterMetrics,
		initHistogramMetrics,
		initObservableGauges,
		registerCallbacks,
	}
	for _, initializer := range initializers {
		if err := initializer(ctx, meter); err != nil {
			log.Error("Failed to initialize memory metrics", "error", err)
			return
		}
	}
	log.Info("Memory metrics initialized successfully")
}

func initCounterMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	for _, counter := range counterMetricSpecs() {
		metricCounter, err := meter.Int64Counter(counter.name, metric.WithDescription(counter.description))
		if err != nil {
			log.Error("Failed to create counter", "name", counter.name, "error", err)
			return err
		}
		*counter.target = metricCounter
	}
	return nil
}

type counterMetricSpec struct {
	name        string
	description string
	target      *metric.Int64Counter
}

func counterMetricSpecs() []counterMetricSpec {
	return []counterMetricSpec{
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "messages_total"),
			"Total number of messages added to memory",
			&memoryMessagesTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "tokens_total"),
			"Total number of tokens added to memory",
			&memoryTokensTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "trim_total"),
			"Total number of memory trim operations",
			&memoryTrimTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "flush_total"),
			"Total number of memory flush operations",
			&memoryFlushTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "lock_acquire_total"),
			"Total number of memory lock acquisitions",
			&memoryLockAcquireTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "lock_contention_total"),
			"Total number of memory lock contentions",
			&memoryLockContentionTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "tokens_saved_total"),
			"Total number of tokens saved by trimming",
			&memoryTokensSavedTotal,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "redaction_operations_total"),
			"Total number of redaction operations",
			&memoryRedactionOperations,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "circuit_breaker_trips_total"),
			"Total number of circuit breaker trips",
			&memoryCircuitBreakerTrips,
		},
	}
}

func initHistogramMetrics(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	var err error
	histogramName := monitoringmetrics.MetricNameWithSubsystem("memory", "operation_duration_seconds")
	memoryOperationLatency, err = meter.Float64Histogram(
		histogramName,
		metric.WithDescription("Duration of memory operations in seconds"),
	)
	if err != nil {
		log.Error("Failed to create histogram", "name", histogramName, "error", err)
		return err
	}
	return nil
}

func initObservableGauges(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	gauges := []struct {
		name        string
		description string
		target      *metric.Int64ObservableGauge
	}{
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "goroutine_pool_active"),
			"Number of active goroutines in memory pools",
			&memoryGoroutinePoolActive,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "tokens_used"),
			"Current number of tokens used in memory instances",
			&memoryTokensUsedGauge,
		},
		{
			monitoringmetrics.MetricNameWithSubsystem("memory", "health_status"),
			"Health status of memory instances (1=healthy, 0=unhealthy)",
			&memoryHealthStatusGauge,
		},
	}
	for _, gauge := range gauges {
		var err error
		*gauge.target, err = meter.Int64ObservableGauge(gauge.name, metric.WithDescription(gauge.description))
		if err != nil {
			log.Error("Failed to create observable gauge", "name", gauge.name, "error", err)
			return err
		}
	}
	return nil
}

func registerCallbacks(ctx context.Context, meter metric.Meter) error {
	callbacks := []func(context.Context, metric.Meter) error{
		registerGoroutinePoolCallback,
		registerTokensUsedCallback,
		registerHealthStatusCallback,
	}
	for _, callback := range callbacks {
		if err := callback(ctx, meter); err != nil {
			return err
		}
	}
	return nil
}

func registerGoroutinePoolCallback(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	if memoryGoroutinePoolActive == nil {
		return nil
	}
	var err error
	goroutinePoolCallback, err = meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			GetDefaultState().RangePoolStates(func(_, value any) bool {
				if poolState, ok := value.(*PoolState); ok {
					poolState.Mu.RLock()
					observer.ObserveInt64(memoryGoroutinePoolActive, poolState.ActiveCount, metric.WithAttributes(
						attribute.String("memory_id", poolState.MemoryID),
					))
					poolState.Mu.RUnlock()
				}
				return true
			})
			return nil
		},
		memoryGoroutinePoolActive,
	)
	if err != nil {
		log.Error("Failed to register goroutine pool callback", "error", err)
		return err
	}
	return nil
}

func registerTokensUsedCallback(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	if memoryTokensUsedGauge == nil {
		return nil
	}
	var err error
	tokensUsedCallback, err = meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			GetDefaultState().RangeTokenStates(func(_, value any) bool {
				if state, ok := value.(*TokenState); ok {
					state.Mu.RLock()
					observer.ObserveInt64(memoryTokensUsedGauge, state.TokensUsed, metric.WithAttributes(
						attribute.String("memory_id", state.MemoryID),
						attribute.Int64("max_tokens", state.MaxTokens),
					))
					state.Mu.RUnlock()
				}
				return true
			})
			return nil
		},
		memoryTokensUsedGauge,
	)
	if err != nil {
		log.Error("Failed to register tokens used callback", "error", err)
		return err
	}
	return nil
}

func registerHealthStatusCallback(ctx context.Context, meter metric.Meter) error {
	log := logger.FromContext(ctx)
	if memoryHealthStatusGauge == nil {
		return nil
	}
	var err error
	healthStatusCallback, err = meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			GetDefaultState().RangeHealthStates(func(_, value any) bool {
				if state, ok := value.(*HealthState); ok {
					state.Mu.RLock()
					healthValue := int64(0)
					if state.IsHealthy {
						healthValue = 1
					}
					observer.ObserveInt64(memoryHealthStatusGauge, healthValue, metric.WithAttributes(
						attribute.String("memory_id", state.MemoryID),
						attribute.Int64("consecutive_failures", int64(state.ConsecutiveFailures)),
					))
					state.Mu.RUnlock()
				}
				return true
			})
			return nil
		},
		memoryHealthStatusGauge,
	)
	if err != nil {
		log.Error("Failed to register health status callback", "error", err)
		return err
	}
	return nil
}

// resetMemoryMetrics resets all memory metrics state
func resetMemoryMetrics(_ context.Context) {
	memoryResetMutex.Lock()
	defer memoryResetMutex.Unlock()

	// Reset state maps
	GetDefaultState().Clear()

	// Unregister callbacks
	if goroutinePoolCallback != nil {
		_ = goroutinePoolCallback.Unregister() //nolint:errcheck // Ignore cleanup errors
		goroutinePoolCallback = nil
	}
	if tokensUsedCallback != nil {
		_ = tokensUsedCallback.Unregister() //nolint:errcheck // Ignore cleanup errors
		tokensUsedCallback = nil
	}
	if healthStatusCallback != nil {
		_ = healthStatusCallback.Unregister() //nolint:errcheck // Ignore cleanup errors
		healthStatusCallback = nil
	}

	// Reset metrics to nil
	memoryMessagesTotal = nil
	memoryTokensTotal = nil
	memoryTrimTotal = nil
	memoryFlushTotal = nil
	memoryLockAcquireTotal = nil
	memoryLockContentionTotal = nil
	memoryTokensSavedTotal = nil
	memoryRedactionOperations = nil
	memoryCircuitBreakerTrips = nil
	memoryOperationLatency = nil
	memoryGoroutinePoolActive = nil
	memoryTokensUsedGauge = nil
	memoryHealthStatusGauge = nil

	// Reset the once flag by creating a new one
	memoryMetricsOnce = sync.Once{}
}

// ResetMemoryMetricsForTesting resets all memory metrics for testing purposes
func ResetMemoryMetricsForTesting(ctx context.Context) {
	resetMemoryMetrics(ctx)
}
