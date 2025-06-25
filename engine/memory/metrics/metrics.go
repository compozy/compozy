package metrics

import (
	"sync"

	"go.opentelemetry.io/otel/metric"
)

// OpenTelemetry metric instruments for memory subsystem monitoring
var (
	// memoryMessagesTotal tracks the total number of messages processed by memory instances
	memoryMessagesTotal metric.Int64Counter
	// memoryTokensTotal tracks the total number of tokens consumed across all operations
	memoryTokensTotal metric.Int64Counter
	// memoryTrimTotal tracks the total number of memory trim operations
	memoryTrimTotal metric.Int64Counter
	// memoryFlushTotal tracks the total number of memory flush operations
	memoryFlushTotal metric.Int64Counter
	// memoryLockAcquireTotal tracks the total number of lock acquisition attempts
	memoryLockAcquireTotal metric.Int64Counter
	// memoryLockContentionTotal tracks the total number of lock contention events
	memoryLockContentionTotal metric.Int64Counter
	// memoryTokensSavedTotal tracks the total number of tokens saved through memory optimization
	memoryTokensSavedTotal metric.Int64Counter
	// memoryRedactionOperations tracks the total number of redaction operations performed
	memoryRedactionOperations metric.Int64Counter
	// memoryCircuitBreakerTrips tracks the total number of circuit breaker trip events
	memoryCircuitBreakerTrips metric.Int64Counter

	// memoryOperationLatency measures the latency distribution of memory operations
	memoryOperationLatency metric.Float64Histogram

	// memoryGoroutinePoolActive tracks the number of active goroutines in memory pools
	memoryGoroutinePoolActive metric.Int64ObservableGauge
	// memoryTokensUsedGauge tracks the current token usage across memory instances
	memoryTokensUsedGauge metric.Int64ObservableGauge
	// memoryHealthStatusGauge tracks the health status of memory instances (0=unhealthy, 1=healthy)
	memoryHealthStatusGauge metric.Int64ObservableGauge

	// goroutinePoolCallback manages the callback registration for goroutine pool metrics
	goroutinePoolCallback metric.Registration
	// tokensUsedCallback manages the callback registration for token usage metrics
	tokensUsedCallback metric.Registration
	// healthStatusCallback manages the callback registration for health status metrics
	healthStatusCallback metric.Registration

	memoryMetricsOnce sync.Once
	memoryResetMutex  sync.Mutex
)
