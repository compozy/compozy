package memory

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Counter metrics
	memoryMessagesTotal       metric.Int64Counter
	memoryTokensTotal         metric.Int64Counter
	memoryTrimTotal           metric.Int64Counter
	memoryFlushTotal          metric.Int64Counter
	memoryLockAcquireTotal    metric.Int64Counter
	memoryLockContentionTotal metric.Int64Counter
	memoryTokensSavedTotal    metric.Int64Counter
	memoryTemporalActivities  metric.Int64Counter
	memoryConfigResolution    metric.Int64Counter
	memoryPrivacyExclusions   metric.Int64Counter
	memoryRedactionOperations metric.Int64Counter
	memoryCircuitBreakerTrips metric.Int64Counter

	// Histogram metrics
	memoryOperationLatency metric.Float64Histogram

	// Gauge metrics
	memoryGoroutinePoolActive metric.Int64ObservableGauge
	memoryTokensUsedGauge     metric.Int64ObservableGauge
	memoryHealthStatusGauge   metric.Int64ObservableGauge

	// Callbacks
	goroutinePoolCallback metric.Registration
	tokensUsedCallback    metric.Registration
	healthStatusCallback  metric.Registration

	// State tracking
	memoryMetricsOnce  sync.Once
	memoryMetricsMutex sync.RWMutex
	memoryPoolStates   sync.Map // map[string]*PoolState
	memoryTokenStates  sync.Map // map[string]*TokenState
	memoryHealthStates sync.Map // map[string]*HealthState
	memoryResetMutex   sync.Mutex
)

// PoolState tracks goroutine pool state for a memory instance
type PoolState struct {
	MemoryID    string
	ActiveCount int64
	MaxPoolSize int64
	mu          sync.RWMutex
}

// TokenState tracks token usage for a memory instance
type TokenState struct {
	MemoryID   string
	TokensUsed int64
	MaxTokens  int64
	mu         sync.RWMutex
}

// HealthState tracks health status for a memory instance
type HealthState struct {
	MemoryID            string
	IsHealthy           bool
	LastHealthCheck     time.Time
	ConsecutiveFailures int
	mu                  sync.RWMutex
}

// InitMemoryMetrics initializes memory system metrics
func InitMemoryMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	log := logger.FromContext(ctx)
	memoryMetricsMutex.Lock()
	defer memoryMetricsMutex.Unlock()
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
			"compozy_memory_temporal_activities_total",
			"Total number of Temporal activities for memory",
			&memoryTemporalActivities,
		},
		{
			"compozy_memory_config_resolution_total",
			"Total number of memory config resolutions",
			&memoryConfigResolution,
		},
		{
			"compozy_memory_privacy_exclusions_total",
			"Total number of messages excluded due to privacy",
			&memoryPrivacyExclusions,
		},
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
		"compozy_memory_operation_latency_seconds",
		metric.WithDescription("Latency of memory operations in seconds"),
	)
	if err != nil {
		log.Error("Failed to create memory operation latency histogram", "error", err)
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
			"Number of active goroutines in memory pool",
			&memoryGoroutinePoolActive,
		},
		{"compozy_memory_tokens_used", "Current number of tokens used in memory", &memoryTokensUsedGauge},
		{
			"compozy_memory_health_status",
			"Memory system health status (1=healthy, 0=unhealthy)",
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
	callbackRegistrators := []func(metric.Meter, logger.Logger) error{
		registerGoroutinePoolCallback,
		registerTokensUsedCallback,
		registerHealthStatusCallback,
	}
	for _, registrator := range callbackRegistrators {
		if err := registrator(meter, log); err != nil {
			return err
		}
	}
	return nil
}

func registerGoroutinePoolCallback(meter metric.Meter, log logger.Logger) error {
	var err error
	goroutinePoolCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		memoryPoolStates.Range(func(key, value any) bool {
			memoryID, ok := key.(string)
			if !ok {
				return true
			}
			state, ok := value.(*PoolState)
			if !ok {
				return true
			}
			state.mu.RLock()
			activeCount := state.ActiveCount
			maxPoolSize := state.MaxPoolSize
			state.mu.RUnlock()
			o.ObserveInt64(memoryGoroutinePoolActive, activeCount,
				metric.WithAttributes(
					attribute.String("memory_id", memoryID),
					attribute.Int64("max_pool_size", maxPoolSize),
				))
			return true
		})
		return nil
	}, memoryGoroutinePoolActive)
	if err != nil {
		log.Error("Failed to register goroutine pool callback", "error", err)
	}
	return err
}

func registerTokensUsedCallback(meter metric.Meter, log logger.Logger) error {
	var err error
	tokensUsedCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		memoryTokenStates.Range(func(key, value any) bool {
			memoryID, ok := key.(string)
			if !ok {
				return true
			}
			state, ok := value.(*TokenState)
			if !ok {
				return true
			}
			state.mu.RLock()
			tokensUsed := state.TokensUsed
			maxTokens := state.MaxTokens
			state.mu.RUnlock()
			var usagePercentage float64
			if maxTokens > 0 {
				usagePercentage = float64(tokensUsed) / float64(maxTokens) * 100
			}
			o.ObserveInt64(memoryTokensUsedGauge, tokensUsed,
				metric.WithAttributes(
					attribute.String("memory_id", memoryID),
					attribute.Int64("max_tokens", maxTokens),
					attribute.Float64("usage_percentage", usagePercentage),
				))
			return true
		})
		return nil
	}, memoryTokensUsedGauge)
	if err != nil {
		log.Error("Failed to register tokens used callback", "error", err)
	}
	return err
}

func registerHealthStatusCallback(meter metric.Meter, log logger.Logger) error {
	var err error
	healthStatusCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		memoryHealthStates.Range(func(key, value any) bool {
			memoryID, ok := key.(string)
			if !ok {
				return true
			}
			state, ok := value.(*HealthState)
			if !ok {
				return true
			}
			state.mu.RLock()
			healthValue := int64(0)
			if state.IsHealthy {
				healthValue = 1
			}
			timeSinceCheck := time.Since(state.LastHealthCheck).Seconds()
			failures := state.ConsecutiveFailures
			state.mu.RUnlock()
			o.ObserveInt64(memoryHealthStatusGauge, healthValue,
				metric.WithAttributes(
					attribute.String("memory_id", memoryID),
					attribute.Float64("time_since_check", timeSinceCheck),
					attribute.Int64("consecutive_failures", int64(failures)),
				))
			return true
		})
		return nil
	}, memoryHealthStatusGauge)
	if err != nil {
		log.Error("Failed to register health status callback", "error", err)
	}
	return err
}

// RecordMemoryMessage records a message being added to memory
func RecordMemoryMessage(ctx context.Context, memoryID string, projectID string, tokens int64) {
	if memoryMessagesTotal != nil {
		memoryMessagesTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
	if memoryTokensTotal != nil && tokens > 0 {
		memoryTokensTotal.Add(ctx, tokens,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
}

// RecordMemoryTrim records a memory trim operation
func RecordMemoryTrim(ctx context.Context, memoryID string, projectID string, strategy string, tokensSaved int64) {
	if memoryTrimTotal != nil {
		memoryTrimTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("strategy", strategy),
				attribute.String("project", projectID),
			))
	}
	if memoryTokensSavedTotal != nil && tokensSaved > 0 {
		memoryTokensSavedTotal.Add(ctx, tokensSaved,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("strategy", strategy),
				attribute.String("project", projectID),
			))
	}
}

// RecordMemoryFlush records a memory flush operation
func RecordMemoryFlush(ctx context.Context, memoryID string, projectID string, flushType string) {
	if memoryFlushTotal != nil {
		memoryFlushTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("type", flushType),
				attribute.String("project", projectID),
			))
	}
}

// RecordMemoryLockAcquire records a lock acquisition
func RecordMemoryLockAcquire(ctx context.Context, memoryID string, projectID string) {
	if memoryLockAcquireTotal != nil {
		memoryLockAcquireTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
}

// RecordMemoryLockContention records lock contention
func RecordMemoryLockContention(ctx context.Context, memoryID string, projectID string) {
	if memoryLockContentionTotal != nil {
		memoryLockContentionTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
}

// RecordMemoryOp records the latency of a memory operation
func RecordMemoryOp(
	ctx context.Context,
	operation string,
	memoryID string,
	projectID string,
	duration time.Duration,
) {
	if memoryOperationLatency != nil {
		memoryOperationLatency.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
}

// RecordTemporalActivity records a Temporal activity execution
func RecordTemporalActivity(ctx context.Context, memoryID string, activityType string, projectID string) {
	if memoryTemporalActivities != nil {
		memoryTemporalActivities.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("activity_type", activityType),
				attribute.String("project", projectID),
			))
	}
}

// RecordConfigResolution records a memory config resolution
func RecordConfigResolution(ctx context.Context, pattern string, projectID string) {
	if memoryConfigResolution != nil {
		memoryConfigResolution.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("pattern", pattern),
				attribute.String("project", projectID),
			))
	}
}

// RecordPrivacyExclusion records a message excluded due to privacy
func RecordPrivacyExclusion(ctx context.Context, memoryID string, reason string, projectID string) {
	if memoryPrivacyExclusions != nil {
		memoryPrivacyExclusions.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("reason", reason),
				attribute.String("project", projectID),
			))
	}
}

// RecordRedactionOperation records a redaction operation
func RecordRedactionOperation(ctx context.Context, memoryID string, fieldCount int64, projectID string) {
	if memoryRedactionOperations != nil {
		memoryRedactionOperations.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
				attribute.Int64("fields_redacted", fieldCount),
			))
	}
}

// RecordCircuitBreakerTrip records a circuit breaker trip
func RecordCircuitBreakerTrip(ctx context.Context, memoryID string, projectID string) {
	if memoryCircuitBreakerTrips != nil {
		memoryCircuitBreakerTrips.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("memory_id", memoryID),
				attribute.String("project", projectID),
			))
	}
}

// UpdateGoroutinePoolState updates the goroutine pool state for a memory instance
func UpdateGoroutinePoolState(memoryID string, activeCount int64, maxPoolSize int64) {
	state := &PoolState{
		MemoryID:    memoryID,
		ActiveCount: activeCount,
		MaxPoolSize: maxPoolSize,
	}
	memoryPoolStates.Store(memoryID, state)
}

// UpdateTokenUsageState updates the token usage state for a memory instance
func UpdateTokenUsageState(memoryID string, tokensUsed int64, maxTokens int64) {
	state := &TokenState{
		MemoryID:   memoryID,
		TokensUsed: tokensUsed,
		MaxTokens:  maxTokens,
	}
	memoryTokenStates.Store(memoryID, state)
}

// UpdateHealthState updates the health state for a memory instance
func UpdateHealthState(memoryID string, isHealthy bool, consecutiveFailures int) {
	value, _ := memoryHealthStates.LoadOrStore(memoryID, &HealthState{
		MemoryID:            memoryID,
		IsHealthy:           isHealthy,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: consecutiveFailures,
	})
	if state, ok := value.(*HealthState); ok {
		state.mu.Lock()
		state.IsHealthy = isHealthy
		state.LastHealthCheck = time.Now()
		state.ConsecutiveFailures = consecutiveFailures
		state.mu.Unlock()
	}
}

// resetMemoryMetrics is used for testing purposes only
func resetMemoryMetrics(ctx context.Context) {
	log := logger.FromContext(ctx)
	// Unregister callbacks
	if goroutinePoolCallback != nil {
		if err := goroutinePoolCallback.Unregister(); err != nil {
			log.Debug("Failed to unregister goroutine pool callback during reset", "error", err)
		}
		goroutinePoolCallback = nil
	}
	if tokensUsedCallback != nil {
		if err := tokensUsedCallback.Unregister(); err != nil {
			log.Debug("Failed to unregister tokens used callback during reset", "error", err)
		}
		tokensUsedCallback = nil
	}
	if healthStatusCallback != nil {
		if err := healthStatusCallback.Unregister(); err != nil {
			log.Debug("Failed to unregister health status callback during reset", "error", err)
		}
		healthStatusCallback = nil
	}
	// Reset all metrics
	memoryMessagesTotal = nil
	memoryTokensTotal = nil
	memoryTrimTotal = nil
	memoryFlushTotal = nil
	memoryLockAcquireTotal = nil
	memoryLockContentionTotal = nil
	memoryTokensSavedTotal = nil
	memoryTemporalActivities = nil
	memoryConfigResolution = nil
	memoryPrivacyExclusions = nil
	memoryRedactionOperations = nil
	memoryCircuitBreakerTrips = nil
	memoryOperationLatency = nil
	memoryGoroutinePoolActive = nil
	memoryTokensUsedGauge = nil
	memoryHealthStatusGauge = nil
	// Clear state maps
	memoryPoolStates = sync.Map{}
	memoryTokenStates = sync.Map{}
	memoryHealthStates = sync.Map{}
	// Reset once
	memoryMetricsOnce = sync.Once{}
}

// ResetMemoryMetricsForTesting resets memory metrics for testing
func ResetMemoryMetricsForTesting(ctx context.Context) {
	memoryResetMutex.Lock()
	defer memoryResetMutex.Unlock()
	resetMemoryMetrics(ctx)
}
