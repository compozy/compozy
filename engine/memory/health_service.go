package memory

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/memory/metrics"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	defaultNearLimitThreshold = 0.85
	defaultCheckInterval      = 30 * time.Second
	defaultHealthTimeout      = 5 * time.Second
)

// HealthService provides centralized health monitoring for the memory system
type HealthService struct {
	healthStates       sync.Map // map[string]*HealthState
	manager            *Manager
	checkInterval      time.Duration
	healthTimeout      time.Duration
	nearLimitThreshold float64
	ctx                context.Context
	stopCh             chan struct{}
	mu                 sync.RWMutex
	lastGlobalCheck    time.Time
}

// SystemHealth represents the overall health status of the memory system
type SystemHealth struct {
	Healthy            bool                       `json:"healthy"`
	TotalInstances     int                        `json:"total_instances"`
	HealthyInstances   int                        `json:"healthy_instances"`
	UnhealthyInstances int                        `json:"unhealthy_instances"`
	LastChecked        time.Time                  `json:"last_checked"`
	InstanceHealth     map[string]*InstanceHealth `json:"instance_health,omitempty"`
	SystemErrors       []string                   `json:"system_errors,omitempty"`
}

// InstanceHealth represents the health status of a specific memory instance
type InstanceHealth struct {
	MemoryID            string            `json:"memory_id"`
	Healthy             bool              `json:"healthy"`
	LastChecked         time.Time         `json:"last_checked"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	ErrorMessage        string            `json:"error_message,omitempty"`
	TokenUsage          *TokenUsageHealth `json:"token_usage,omitempty"`
}

// TokenUsageHealth represents token usage health metrics
type TokenUsageHealth struct {
	Used            int     `json:"used"`
	MaxTokens       int     `json:"max_tokens"`
	UsagePercentage float64 `json:"usage_percentage"`
	NearLimit       bool    `json:"near_limit"`
}

// HealthServiceOptions contains configuration options for the health service
type HealthServiceOptions struct {
	CheckInterval      time.Duration
	HealthTimeout      time.Duration
	NearLimitThreshold float64
}

// NewHealthService creates a new memory health service with default options
func NewHealthService(ctx context.Context, manager *Manager) *HealthService {
	return NewHealthServiceWithOptions(ctx, manager, nil)
}

// NewHealthServiceWithOptions creates a new memory health service with custom options
func NewHealthServiceWithOptions(ctx context.Context, manager *Manager, opts *HealthServiceOptions) *HealthService {
	ctx = logger.ContextWithLogger(ctx, logger.FromContext(ctx).With("component", "memory_health_service"))
	if opts == nil {
		opts = &HealthServiceOptions{
			CheckInterval:      defaultCheckInterval,
			HealthTimeout:      defaultHealthTimeout,
			NearLimitThreshold: defaultNearLimitThreshold,
		}
	}
	return &HealthService{
		manager:            manager,
		checkInterval:      opts.CheckInterval,
		healthTimeout:      opts.HealthTimeout,
		nearLimitThreshold: opts.NearLimitThreshold,
		ctx:                ctx,
		stopCh:             make(chan struct{}),
	}
}

// Start begins the health monitoring service
func (mhs *HealthService) Start(ctx context.Context) {
	go mhs.healthCheckLoop(ctx)
	logger.FromContext(ctx).Info("Memory health service started", "check_interval", mhs.checkInterval)
}

// Stop shuts down the health monitoring service
func (mhs *HealthService) Stop() {
	close(mhs.stopCh)
	logger.FromContext(mhs.ctx).Info("Memory health service stopped")
}

// GetOverallHealth returns the overall health status of the memory system
func (mhs *HealthService) GetOverallHealth(ctx context.Context) *SystemHealth {
	mhs.ensureRecentHealthCheck(ctx)
	health := mhs.initializeSystemHealth()
	var totalInstances, healthyInstances int
	// Only collect instance health if we have a manager
	if mhs.manager != nil {
		totalInstances, healthyInstances = mhs.collectInstanceHealth(health)
	}
	mhs.finalizeSystemHealth(health, totalInstances, healthyInstances)
	mhs.updateLastGlobalCheck()
	return health
}

// ensureRecentHealthCheck performs a health check if the last check is stale
func (mhs *HealthService) ensureRecentHealthCheck(ctx context.Context) {
	mhs.mu.RLock()
	lastCheck := mhs.lastGlobalCheck
	mhs.mu.RUnlock()
	if time.Since(lastCheck) > mhs.checkInterval {
		mhs.performHealthCheck(ctx)
	}
}

// initializeSystemHealth creates a new SystemHealth instance with default values
func (mhs *HealthService) initializeSystemHealth() *SystemHealth {
	return &SystemHealth{
		InstanceHealth: make(map[string]*InstanceHealth),
		LastChecked:    time.Now(),
		SystemErrors:   []string{},
	}
}

// collectInstanceHealth iterates through all tracked instances and collects their health status
func (mhs *HealthService) collectInstanceHealth(health *SystemHealth) (totalInstances, healthyInstances int) {
	mhs.healthStates.Range(func(key, value any) bool {
		memoryID, ok := key.(string)
		if !ok {
			return true
		}
		state, ok := value.(*metrics.HealthState)
		if !ok {
			return true
		}
		totalInstances++
		instanceHealth := mhs.buildInstanceHealth(memoryID, state)
		if instanceHealth.Healthy {
			healthyInstances++
		}
		health.InstanceHealth[memoryID] = instanceHealth
		return true
	})
	return totalInstances, healthyInstances
}

// buildInstanceHealth creates an InstanceHealth object for a specific memory instance
func (mhs *HealthService) buildInstanceHealth(memoryID string, state *metrics.HealthState) *InstanceHealth {
	// Read state fields under lock to prevent races
	state.Mu.RLock()
	healthy := state.IsHealthy
	lastCheck := state.LastHealthCheck
	failures := state.ConsecutiveFailures
	state.Mu.RUnlock()

	instanceHealth := &InstanceHealth{
		MemoryID:            memoryID,
		LastChecked:         lastCheck,
		ConsecutiveFailures: failures,
	}
	mhs.setInstanceHealthStatus(instanceHealth, healthy, lastCheck)
	mhs.addTokenUsageInfo(instanceHealth, memoryID)
	return instanceHealth
}

// setInstanceHealthStatus determines if an instance is healthy based on its state
func (mhs *HealthService) setInstanceHealthStatus(instanceHealth *InstanceHealth, healthy bool, lastCheck time.Time) {
	if healthy && time.Since(lastCheck) < 2*mhs.checkInterval {
		instanceHealth.Healthy = true
	} else {
		instanceHealth.Healthy = false
		if time.Since(lastCheck) > 2*mhs.checkInterval {
			instanceHealth.ErrorMessage = "health check timeout"
		}
	}
}

// addTokenUsageInfo adds token usage information to instance health if available
func (mhs *HealthService) addTokenUsageInfo(instanceHealth *InstanceHealth, memoryID string) {
	ts, exists := metrics.GetDefaultState().GetTokenState(memoryID)
	if !exists {
		return
	}
	ts.Mu.RLock()
	defer ts.Mu.RUnlock()
	var usagePercentage float64
	var nearLimit bool
	if ts.MaxTokens > 0 {
		usagePercentage = float64(ts.TokensUsed) / float64(ts.MaxTokens) * 100
		nearLimit = usagePercentage > mhs.nearLimitThreshold*100
	}
	instanceHealth.TokenUsage = &TokenUsageHealth{
		Used:            int(ts.TokensUsed),
		MaxTokens:       int(ts.MaxTokens),
		UsagePercentage: usagePercentage,
		NearLimit:       nearLimit,
	}
}

// finalizeSystemHealth sets the final health status and adds system-level errors
func (mhs *HealthService) finalizeSystemHealth(health *SystemHealth, totalInstances, healthyInstances int) {
	health.TotalInstances = totalInstances
	health.HealthyInstances = healthyInstances
	health.UnhealthyInstances = totalInstances - healthyInstances
	if mhs.manager == nil {
		health.SystemErrors = append(health.SystemErrors, "memory manager not available")
		health.Healthy = false
	} else {
		health.Healthy = totalInstances > 0 && healthyInstances == totalInstances
	}
}

// updateLastGlobalCheck updates the timestamp of the last global health check
func (mhs *HealthService) updateLastGlobalCheck() {
	mhs.mu.Lock()
	mhs.lastGlobalCheck = time.Now()
	mhs.mu.Unlock()
}

// GetInstanceHealth returns the health status of a specific memory instance
func (mhs *HealthService) GetInstanceHealth(memoryID string) (*InstanceHealth, bool) {
	value, exists := mhs.healthStates.Load(memoryID)
	if !exists {
		return nil, false
	}

	state, ok := value.(*metrics.HealthState)
	if !ok {
		return nil, false
	}

	// Read state fields under lock to prevent races
	state.Mu.RLock()
	healthy := state.IsHealthy
	lastCheck := state.LastHealthCheck
	failures := state.ConsecutiveFailures
	state.Mu.RUnlock()

	instanceHealth := &InstanceHealth{
		MemoryID:            memoryID,
		LastChecked:         lastCheck,
		ConsecutiveFailures: failures,
		Healthy:             healthy && time.Since(lastCheck) < 2*mhs.checkInterval,
	}

	if !instanceHealth.Healthy && time.Since(lastCheck) > 2*mhs.checkInterval {
		instanceHealth.ErrorMessage = "health check timeout"
	}

	return instanceHealth, true
}

// RegisterInstance registers a memory instance for health monitoring
func (mhs *HealthService) RegisterInstance(memoryID string) {
	state := &metrics.HealthState{
		MemoryID:            memoryID,
		IsHealthy:           true,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: 0,
	}

	mhs.healthStates.Store(memoryID, state)
	logger.FromContext(mhs.ctx).Debug("Registered memory instance for health monitoring", "memory_id", memoryID)
}

// UnregisterInstance removes a memory instance from health monitoring
func (mhs *HealthService) UnregisterInstance(memoryID string) {
	mhs.healthStates.Delete(memoryID)
	logger.FromContext(mhs.ctx).Debug("Unregistered memory instance from health monitoring", "memory_id", memoryID)
}

// healthCheckLoop performs periodic health checks
func (mhs *HealthService) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(mhs.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mhs.stopCh:
			return
		case <-ticker.C:
			mhs.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck performs a health check on all registered instances
func (mhs *HealthService) performHealthCheck(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, mhs.healthTimeout)
	defer cancel()

	mhs.healthStates.Range(func(key, value any) bool {
		memoryID, ok := key.(string)
		if !ok {
			return true
		}

		state, ok := value.(*metrics.HealthState)
		if !ok {
			return true
		}

		// Perform health check (this could be expanded to actually ping the instance)
		healthy := mhs.checkInstanceHealth(checkCtx, memoryID)

		state.Mu.Lock()
		state.LastHealthCheck = time.Now()
		if healthy {
			state.IsHealthy = true
			state.ConsecutiveFailures = 0
		} else {
			state.IsHealthy = false
			state.ConsecutiveFailures++
		}
		// Capture the failures count before releasing the lock
		failures := state.ConsecutiveFailures
		state.Mu.Unlock()

		// Update the global health state
		metrics.GetDefaultState().UpdateHealthState(memoryID, healthy, failures)

		return true
	})
}

// checkInstanceHealth performs a health check on a specific instance
func (mhs *HealthService) checkInstanceHealth(ctx context.Context, memoryID string) bool {
	if mhs.manager == nil {
		return false
	}

	// For now, we consider an instance healthy based on its recent activity
	// and basic connectivity checks. Since we can't easily access the actual
	// instance without proper workflow context, we'll rely on the health state
	// tracking that's updated when instances perform operations.

	// Check if the instance has been recently active
	if healthState, exists := metrics.GetDefaultState().GetHealthState(memoryID); exists {
		healthState.Mu.RLock()
		timeSinceCheck := time.Since(healthState.LastHealthCheck)
		isHealthy := healthState.IsHealthy
		consecutiveFailures := healthState.ConsecutiveFailures
		healthState.Mu.RUnlock()

		// Consider unhealthy if:
		// 1. The instance is marked as unhealthy
		// 2. Not checked in over 5 minutes
		// 3. Has too many consecutive failures
		if !isHealthy {
			logger.FromContext(mhs.ctx).Debug("Instance marked as unhealthy",
				"memory_id", memoryID,
				"consecutive_failures", consecutiveFailures)
			return false
		}

		if timeSinceCheck > 5*time.Minute {
			logger.FromContext(mhs.ctx).Debug("Instance inactive for too long",
				"memory_id", memoryID,
				"time_since_check", timeSinceCheck)
			return false
		}

		if consecutiveFailures > 3 {
			logger.FromContext(mhs.ctx).Debug("Too many consecutive failures",
				"memory_id", memoryID,
				"failures", consecutiveFailures)
			return false
		}

		// Perform comprehensive operational health checks
		if !mhs.performOperationalChecks(ctx, memoryID) {
			return false
		}

		return true
	}

	// If we don't have health state for this instance, default to healthy
	// This allows new instances to be considered healthy until proven otherwise
	logger.FromContext(mhs.ctx).Debug(
		"No health state found for instance, defaulting to healthy",
		"memory_id", memoryID,
	)
	return true
}

// performOperationalChecks performs comprehensive operational health checks
func (mhs *HealthService) performOperationalChecks(ctx context.Context, memoryID string) bool {
	// Check Redis connectivity and operations if available
	if mhs.manager.baseRedisClient != nil {
		// 1. Basic connectivity check
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := mhs.manager.baseRedisClient.Ping(pingCtx).Err(); err != nil {
			logger.FromContext(mhs.ctx).Debug("Instance Redis connectivity check failed",
				"memory_id", memoryID,
				"error", err)
			return false
		}
		// 2. Test basic read/write operations
		testKey := fmt.Sprintf("health:check:%s:%d:%s", memoryID, time.Now().UnixNano(), generateRandomString(8))
		testValue := "health-check-value"
		// Write test with timeout
		writeCtx, writeCancel := context.WithTimeout(ctx, 2*time.Second)
		defer writeCancel()
		if err := mhs.manager.baseRedisClient.Set(writeCtx, testKey, testValue, 10*time.Second).Err(); err != nil {
			logger.FromContext(mhs.ctx).Debug("Health check write operation failed",
				"memory_id", memoryID,
				"error", err)
			return false
		}
		// Read test with timeout
		readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
		defer readCancel()
		val, err := mhs.manager.baseRedisClient.Get(readCtx, testKey).Result()
		if err != nil || val != testValue {
			logger.FromContext(mhs.ctx).Debug("Health check read operation failed",
				"memory_id", memoryID,
				"error", err,
				"expected", testValue,
				"got", val)
			return false
		}
		// Cleanup test key
		delCtx, delCancel := context.WithTimeout(ctx, 1*time.Second)
		defer delCancel()
		if err := mhs.manager.baseRedisClient.Del(delCtx, testKey).Err(); err != nil {
			logger.FromContext(mhs.ctx).Debug("Failed to cleanup health check test key",
				"memory_id", memoryID,
				"key", testKey,
				"error", err)
		}
	}
	// 3. Test lock operations if lock manager is available
	if mhs.manager.baseLockManager != nil {
		lockKey := fmt.Sprintf("health:lock:%s:%d:%s", memoryID, time.Now().UnixNano(), generateRandomString(8))
		lockCtx, lockCancel := context.WithTimeout(ctx, 3*time.Second)
		defer lockCancel()
		lock, err := mhs.manager.baseLockManager.Acquire(lockCtx, lockKey, 5*time.Second)
		if err != nil {
			logger.FromContext(mhs.ctx).Debug("Health check lock acquisition failed",
				"memory_id", memoryID,
				"error", err)
			return false
		}
		// Release the lock
		releaseCtx, releaseCancel := context.WithTimeout(ctx, 1*time.Second)
		defer releaseCancel()
		if err := lock.Release(releaseCtx); err != nil {
			logger.FromContext(mhs.ctx).Debug("Health check lock release failed",
				"memory_id", memoryID,
				"error", err)
			return false
		}
	}
	logger.FromContext(mhs.ctx).Debug("Operational health checks passed", "memory_id", memoryID)
	return true
}

// SetCheckInterval sets the health check interval
func (mhs *HealthService) SetCheckInterval(interval time.Duration) {
	mhs.checkInterval = interval
}

// SetHealthTimeout sets the timeout for individual health checks
func (mhs *HealthService) SetHealthTimeout(timeout time.Duration) {
	mhs.healthTimeout = timeout
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to timestamp-based selection if crypto/rand fails
			result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			result[i] = charset[num.Int64()]
		}
	}
	return string(result)
}
