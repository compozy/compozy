package memory

import (
	"context"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// HealthService provides centralized health monitoring for the memory system
type HealthService struct {
	healthStates    sync.Map // map[string]*HealthState
	manager         *Manager
	checkInterval   time.Duration
	healthTimeout   time.Duration
	log             logger.Logger
	stopCh          chan struct{}
	mu              sync.RWMutex
	lastGlobalCheck time.Time
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

// NewHealthService creates a new memory health service
func NewHealthService(manager *Manager, log logger.Logger) *HealthService {
	if log == nil {
		log = logger.NewForTests()
	}

	return &HealthService{
		manager:       manager,
		checkInterval: 30 * time.Second, // Check every 30 seconds
		healthTimeout: 5 * time.Second,  // Timeout for individual health checks
		log:           log.With("component", "memory_health_service"),
		stopCh:        make(chan struct{}),
	}
}

// Start begins the health monitoring service
func (mhs *HealthService) Start(ctx context.Context) {
	go mhs.healthCheckLoop(ctx)
	mhs.log.Info("Memory health service started", "check_interval", mhs.checkInterval)
}

// Stop shuts down the health monitoring service
func (mhs *HealthService) Stop() {
	close(mhs.stopCh)
	mhs.log.Info("Memory health service stopped")
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
		state, ok := value.(*HealthState)
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
func (mhs *HealthService) buildInstanceHealth(memoryID string, state *HealthState) *InstanceHealth {
	// Read state fields under lock to prevent races
	state.mu.RLock()
	healthy := state.IsHealthy
	lastCheck := state.LastHealthCheck
	failures := state.ConsecutiveFailures
	state.mu.RUnlock()

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
	tokenState, exists := memoryTokenStates.Load(memoryID)
	if !exists {
		return
	}
	ts, ok := tokenState.(*TokenState)
	if !ok {
		return
	}
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	var usagePercentage float64
	var nearLimit bool
	if ts.MaxTokens > 0 {
		usagePercentage = float64(ts.TokensUsed) / float64(ts.MaxTokens) * 100
		nearLimit = usagePercentage > 85.0
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

	state, ok := value.(*HealthState)
	if !ok {
		return nil, false
	}

	// Read state fields under lock to prevent races
	state.mu.RLock()
	healthy := state.IsHealthy
	lastCheck := state.LastHealthCheck
	failures := state.ConsecutiveFailures
	state.mu.RUnlock()

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
	state := &HealthState{
		MemoryID:            memoryID,
		IsHealthy:           true,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: 0,
	}

	mhs.healthStates.Store(memoryID, state)
	mhs.log.Debug("Registered memory instance for health monitoring", "memory_id", memoryID)
}

// UnregisterInstance removes a memory instance from health monitoring
func (mhs *HealthService) UnregisterInstance(memoryID string) {
	mhs.healthStates.Delete(memoryID)
	mhs.log.Debug("Unregistered memory instance from health monitoring", "memory_id", memoryID)
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

		state, ok := value.(*HealthState)
		if !ok {
			return true
		}

		// Perform health check (this could be expanded to actually ping the instance)
		healthy := mhs.checkInstanceHealth(checkCtx, memoryID)

		state.mu.Lock()
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
		state.mu.Unlock()

		// Update the global health state
		UpdateHealthState(memoryID, healthy, failures)

		return true
	})
}

// checkInstanceHealth performs a health check on a specific instance
func (mhs *HealthService) checkInstanceHealth(_ context.Context, memoryID string) bool {
	if mhs.manager == nil {
		return false
	}

	// This is a simple check - in a real implementation, you might want to
	// actually create an instance and test basic operations
	// For now, we'll consider an instance healthy if it's been recently active
	if state, exists := memoryHealthStates.Load(memoryID); exists {
		if healthState, ok := state.(*HealthState); ok {
			healthState.mu.RLock()
			timeSinceCheck := time.Since(healthState.LastHealthCheck)
			healthState.mu.RUnlock()

			// Consider healthy if checked within the last 5 minutes
			return timeSinceCheck < 5*time.Minute
		}
	}

	return true // Default to healthy for new instances
}

// SetCheckInterval sets the health check interval
func (mhs *HealthService) SetCheckInterval(interval time.Duration) {
	mhs.checkInterval = interval
}

// SetHealthTimeout sets the timeout for individual health checks
func (mhs *HealthService) SetHealthTimeout(timeout time.Duration) {
	mhs.healthTimeout = timeout
}
