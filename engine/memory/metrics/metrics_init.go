package metrics

import (
	"context"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// InitMemoryMetrics initializes memory system metrics
func InitMemoryMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	log := logger.FromContext(ctx)
	memoryMetricsOnce.Do(func() {
		performMetricsInitialization(meter, log)
	})
}

func performMetricsInitialization(meter metric.Meter, log logger.Logger) {
	initializers := []func(metric.Meter, logger.Logger) error{
		initCounterMetrics,
		initHistogramMetrics,
		initObservableGauges,
		registerCallbacks,
	}
	for _, initializer := range initializers {
		if err := initializer(meter, log); err != nil {
			log.Error("Failed to initialize memory metrics", "error", err)
			return
		}
	}
	log.Info("Memory metrics initialized successfully")
}

func initCounterMetrics(meter metric.Meter, log logger.Logger) error {
	counters := []struct {
		name        string
		description string
		target      *metric.Int64Counter
	}{
		{"compozy_memory_messages_total", "Total number of messages added to memory", &memoryMessagesTotal},
		{"compozy_memory_tokens_total", "Total number of tokens added to memory", &memoryTokensTotal},
		{"compozy_memory_trim_total", "Total number of memory trim operations", &memoryTrimTotal},
		{"compozy_memory_flush_total", "Total number of memory flush operations", &memoryFlushTotal},
		{"compozy_memory_lock_acquire_total", "Total number of memory lock acquisitions", &memoryLockAcquireTotal},
		{"compozy_memory_lock_contention_total", "Total number of memory lock contentions", &memoryLockContentionTotal},
		{"compozy_memory_tokens_saved_total", "Total number of tokens saved by trimming", &memoryTokensSavedTotal},
		{
			"compozy_memory_redaction_operations_total",
			"Total number of redaction operations",
			&memoryRedactionOperations,
		},
		{
			"compozy_memory_circuit_breaker_trips_total",
			"Total number of circuit breaker trips",
			&memoryCircuitBreakerTrips,
		},
	}
	for _, counter := range counters {
		var err error
		*counter.target, err = meter.Int64Counter(counter.name, metric.WithDescription(counter.description))
		if err != nil {
			log.Error("Failed to create counter", "name", counter.name, "error", err)
			return err
		}
	}
	return nil
}

func initHistogramMetrics(meter metric.Meter, log logger.Logger) error {
	var err error
	memoryOperationLatency, err = meter.Float64Histogram(
		"compozy_memory_operation_duration_seconds",
		metric.WithDescription("Duration of memory operations in seconds"),
	)
	if err != nil {
		log.Error("Failed to create histogram", "name", "compozy_memory_operation_duration_seconds", "error", err)
		return err
	}
	return nil
}

func initObservableGauges(meter metric.Meter, log logger.Logger) error {
	gauges := []struct {
		name        string
		description string
		target      *metric.Int64ObservableGauge
	}{
		{
			"compozy_memory_goroutine_pool_active",
			"Number of active goroutines in memory pools",
			&memoryGoroutinePoolActive,
		},
		{
			"compozy_memory_tokens_used",
			"Current number of tokens used in memory instances",
			&memoryTokensUsedGauge,
		},
		{
			"compozy_memory_health_status",
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

func registerCallbacks(meter metric.Meter, log logger.Logger) error {
	callbacks := []func(metric.Meter, logger.Logger) error{
		registerGoroutinePoolCallback,
		registerTokensUsedCallback,
		registerHealthStatusCallback,
	}
	for _, callback := range callbacks {
		if err := callback(meter, log); err != nil {
			return err
		}
	}
	return nil
}

func registerGoroutinePoolCallback(meter metric.Meter, log logger.Logger) error {
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

func registerTokensUsedCallback(meter metric.Meter, log logger.Logger) error {
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

func registerHealthStatusCallback(meter metric.Meter, log logger.Logger) error {
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
