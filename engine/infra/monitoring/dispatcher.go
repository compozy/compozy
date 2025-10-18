package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring/metrics"
	"github.com/compozy/compozy/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	dispatcherHealthGauge         metric.Int64ObservableGauge
	dispatcherHeartbeatAgeSeconds metric.Float64ObservableGauge
	dispatcherFailureCount        metric.Int64ObservableGauge
	dispatcherHealthCallback      metric.Registration
	dispatcherHealthInitOnce      sync.Once
	dispatcherHealthMutex         sync.RWMutex
	dispatcherHealthStore         sync.Map // map[string]DispatcherHealth
	dispatcherHealthResetMutex    sync.Mutex
)

// DispatcherHealth represents the health status of a dispatcher
type DispatcherHealth struct {
	DispatcherID        string
	LastHeartbeat       time.Time
	IsHealthy           bool
	StaleThreshold      time.Duration
	LastHealthCheck     time.Time
	ConsecutiveFailures int
	mu                  sync.RWMutex // Protects all fields for concurrent access
}

// IsStale returns true if the dispatcher hasn't sent a heartbeat within the stale threshold
func (dh *DispatcherHealth) IsStale() bool {
	dh.mu.RLock()
	defer dh.mu.RUnlock()
	return time.Since(dh.LastHeartbeat) > dh.StaleThreshold
}

// UpdateHealth updates the health status based on current time and stale threshold
func (dh *DispatcherHealth) UpdateHealth() {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	dh.LastHealthCheck = time.Now()
	wasHealthy := dh.IsHealthy
	isStale := time.Since(dh.LastHeartbeat) > dh.StaleThreshold
	dh.IsHealthy = !isStale
	if !dh.IsHealthy {
		if wasHealthy {
			// Transition from healthy to unhealthy
			dh.ConsecutiveFailures = 1
		} else {
			// Continue being unhealthy
			dh.ConsecutiveFailures++
		}
	} else if dh.IsHealthy {
		dh.ConsecutiveFailures = 0
	}
}

// getMetricValues safely retrieves all metric values for observation
func (dh *DispatcherHealth) getMetricValues(
	now time.Time,
) (healthValue int64, isStale bool, timeSinceHeartbeat float64, consecutiveFailures int) {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	if dh.IsHealthy {
		healthValue = 1
	}
	isStale = now.Sub(dh.LastHeartbeat) > dh.StaleThreshold
	timeSinceHeartbeat = now.Sub(dh.LastHeartbeat).Seconds()
	consecutiveFailures = dh.ConsecutiveFailures
	return
}

// initDispatcherHealthMetrics initializes dispatcher health monitoring metrics
func initDispatcherHealthMetrics(ctx context.Context, meter metric.Meter) {
	if meter == nil {
		return
	}
	log := logger.FromContext(ctx)
	dispatcherHealthMutex.Lock()
	defer dispatcherHealthMutex.Unlock()
	dispatcherHealthInitOnce.Do(func() {
		var err error
		dispatcherHealthGauge, err = meter.Int64ObservableGauge(
			metrics.MetricNameWithSubsystem("dispatcher", "health_status"),
			metric.WithDescription("Dispatcher health status (1=healthy, 0=unhealthy)"),
		)
		if err != nil {
			log.Error("Failed to create dispatcher health gauge", "error", err, "component", "dispatcher_health")
			return
		}
		dispatcherHeartbeatAgeSeconds, err = meter.Float64ObservableGauge(
			metrics.MetricNameWithSubsystem("dispatcher", "heartbeat_age_seconds"),
			metric.WithDescription("Seconds since the last dispatcher heartbeat was observed"),
		)
		if err != nil {
			log.Error("Failed to create dispatcher heartbeat age gauge", "error", err, "component", "dispatcher_health")
			return
		}
		dispatcherFailureCount, err = meter.Int64ObservableGauge(
			metrics.MetricNameWithSubsystem("dispatcher", "consecutive_failures"),
			metric.WithDescription("Number of consecutive dispatcher health check failures"),
		)
		if err != nil {
			log.Error("Failed to create dispatcher failure count gauge", "error", err, "component", "dispatcher_health")
			return
		}
		// Register callback for health status reporting
		dispatcherHealthCallback, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
			now := time.Now()
			dispatcherHealthStore.Range(func(key, value any) bool {
				dispatcherID, ok := key.(string)
				if !ok {
					return true // Skip invalid key
				}
				health, ok := value.(*DispatcherHealth)
				if !ok {
					return true // Skip invalid value
				}
				// Update health status before reporting
				health.UpdateHealth()
				// Get all metric values safely at once
				healthValue, isStale, timeSinceHeartbeat, failures := health.getMetricValues(now)
				o.ObserveInt64(dispatcherHealthGauge, healthValue,
					metric.WithAttributes(
						attribute.String("dispatcher_id", dispatcherID),
						attribute.Bool("is_stale", isStale),
					))
				o.ObserveFloat64(dispatcherHeartbeatAgeSeconds, timeSinceHeartbeat,
					metric.WithAttributes(
						attribute.String("dispatcher_id", dispatcherID),
					))
				o.ObserveInt64(dispatcherFailureCount, int64(failures),
					metric.WithAttributes(
						attribute.String("dispatcher_id", dispatcherID),
					))
				return true
			})
			return nil
		}, dispatcherHealthGauge, dispatcherHeartbeatAgeSeconds, dispatcherFailureCount)
		if err != nil {
			log.Error("Failed to register dispatcher health callback", "error", err, "component", "dispatcher_health")
		}
	})
}

// InitDispatcherHealthMetrics initializes dispatcher health monitoring
func InitDispatcherHealthMetrics(ctx context.Context, meter metric.Meter) {
	initDispatcherHealthMetrics(ctx, meter)
}

// RegisterDispatcher registers a new dispatcher for health monitoring
func RegisterDispatcher(ctx context.Context, dispatcherID string, staleThreshold time.Duration) {
	if staleThreshold == 0 {
		staleThreshold = 2 * time.Minute // Default stale threshold
	}
	health := &DispatcherHealth{
		DispatcherID:        dispatcherID,
		LastHeartbeat:       time.Now(),
		IsHealthy:           true,
		StaleThreshold:      staleThreshold,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: 0,
	}
	dispatcherHealthStore.Store(dispatcherID, health)
	log := logger.FromContext(ctx)
	log.Debug("Registered dispatcher for health monitoring",
		"dispatcher_id", dispatcherID,
		"stale_threshold", staleThreshold)
}

// UnregisterDispatcher removes a dispatcher from health monitoring
func UnregisterDispatcher(ctx context.Context, dispatcherID string) {
	dispatcherHealthStore.Delete(dispatcherID)
	log := logger.FromContext(ctx)
	log.Debug("Unregistered dispatcher from health monitoring", "dispatcher_id", dispatcherID)
}

// UpdateDispatcherHeartbeat updates the last heartbeat time for a dispatcher
func UpdateDispatcherHeartbeat(ctx context.Context, dispatcherID string) {
	value, ok := dispatcherHealthStore.Load(dispatcherID)
	if !ok {
		return
	}
	health, ok := value.(*DispatcherHealth)
	if !ok {
		return // Skip invalid value
	}

	health.mu.Lock()
	defer health.mu.Unlock()

	health.LastHeartbeat = time.Now()

	// Atomically re-evaluate health status based on the new heartbeat
	health.LastHealthCheck = time.Now()
	health.IsHealthy = true // A fresh heartbeat always means it's healthy
	health.ConsecutiveFailures = 0
	isHealthy := health.IsHealthy

	log := logger.FromContext(ctx)
	log.Debug("Updated dispatcher heartbeat",
		"dispatcher_id", dispatcherID,
		"is_healthy", isHealthy)
}

// GetDispatcherHealth returns the health status for a specific dispatcher
func GetDispatcherHealth(dispatcherID string) (*DispatcherHealth, bool) {
	if value, ok := dispatcherHealthStore.Load(dispatcherID); ok {
		health, ok := value.(*DispatcherHealth)
		if !ok {
			return nil, false // Invalid value
		}
		health.UpdateHealth()
		return health, true
	}
	return nil, false
}

// GetAllDispatcherHealth returns health status for all registered dispatchers
func GetAllDispatcherHealth() map[string]*DispatcherHealth {
	result := make(map[string]*DispatcherHealth)
	dispatcherHealthStore.Range(func(key, value any) bool {
		dispatcherID, ok := key.(string)
		if !ok {
			return true // Skip invalid key
		}
		health, ok := value.(*DispatcherHealth)
		if !ok {
			return true // Skip invalid value
		}
		health.UpdateHealth()
		result[dispatcherID] = health
		return true
	})
	return result
}

// GetHealthyDispatcherCount returns the count of healthy dispatchers
func GetHealthyDispatcherCount() int {
	count := 0
	dispatcherHealthStore.Range(func(_, value any) bool {
		health, ok := value.(*DispatcherHealth)
		if !ok {
			return true // Skip invalid value
		}
		health.UpdateHealth()
		if health.IsHealthy {
			count++
		}
		return true
	})
	return count
}

// GetStaleDispatcherCount returns the count of stale dispatchers
func GetStaleDispatcherCount() int {
	count := 0
	dispatcherHealthStore.Range(func(_, value any) bool {
		health, ok := value.(*DispatcherHealth)
		if !ok {
			return true // Skip invalid value
		}
		health.UpdateHealth()
		if health.IsStale() {
			count++
		}
		return true
	})
	return count
}

// resetDispatcherHealthMetrics is used for testing purposes only
func resetDispatcherHealthMetrics(ctx context.Context) {
	if dispatcherHealthCallback != nil {
		err := dispatcherHealthCallback.Unregister()
		if err != nil {
			log := logger.FromContext(ctx)
			log.Debug(
				"Failed to unregister dispatcher health callback during reset",
				"error",
				err,
				"component",
				"dispatcher_health",
			)
		}
		dispatcherHealthCallback = nil
	}
	dispatcherHealthGauge = nil
	dispatcherHealthStore = sync.Map{}
	dispatcherHealthInitOnce = sync.Once{}
}

// ResetDispatcherHealthMetricsForTesting resets dispatcher health metrics for testing
// This should only be used in tests to ensure clean state between test runs
func ResetDispatcherHealthMetricsForTesting(ctx context.Context) {
	dispatcherHealthResetMutex.Lock()
	defer dispatcherHealthResetMutex.Unlock()
	resetDispatcherHealthMetrics(ctx)
}
