package metrics

import (
	"sync"

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
	MemoryPoolStates   sync.Map // map[string]*PoolState
	MemoryTokenStates  sync.Map // map[string]*TokenState
	MemoryHealthStates sync.Map // map[string]*HealthState
	memoryResetMutex   sync.Mutex
)

// Metric functions and state management are now in:
// - metrics_init.go: Initialization functions
// - metrics_recording.go: Recording functions
// - metrics_state.go: State structures and management
