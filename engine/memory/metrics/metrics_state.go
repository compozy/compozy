package metrics

import (
	"sync"
	"time"
)

// PoolState tracks goroutine pool state for a memory instance
type PoolState struct {
	MemoryID    string
	ActiveCount int64
	MaxPoolSize int64
	Mu          sync.RWMutex
}

// TokenState tracks token usage for a memory instance
type TokenState struct {
	MemoryID   string
	TokensUsed int64
	MaxTokens  int64
	Mu         sync.RWMutex
}

// HealthState tracks health status for a memory instance
type HealthState struct {
	MemoryID            string
	IsHealthy           bool
	LastHealthCheck     time.Time
	ConsecutiveFailures int
	Mu                  sync.RWMutex
}

// UpdateGoroutinePoolState updates the goroutine pool state for tracking
func UpdateGoroutinePoolState(memoryID string, activeCount int64, maxPoolSize int64) {
	state, _ := MemoryPoolStates.LoadOrStore(memoryID, &PoolState{
		MemoryID:    memoryID,
		ActiveCount: activeCount,
		MaxPoolSize: maxPoolSize,
	})
	if poolState, ok := state.(*PoolState); ok {
		poolState.Mu.Lock()
		poolState.ActiveCount = activeCount
		poolState.MaxPoolSize = maxPoolSize
		poolState.Mu.Unlock()
	}
}

// UpdateTokenUsageState updates the token usage state for tracking
func UpdateTokenUsageState(memoryID string, tokensUsed int64, maxTokens int64) {
	state, _ := MemoryTokenStates.LoadOrStore(memoryID, &TokenState{
		MemoryID:   memoryID,
		TokensUsed: tokensUsed,
		MaxTokens:  maxTokens,
	})
	if tokenState, ok := state.(*TokenState); ok {
		tokenState.Mu.Lock()
		tokenState.TokensUsed = tokensUsed
		tokenState.MaxTokens = maxTokens
		tokenState.Mu.Unlock()
	}
}

// UpdateHealthState updates the health state for tracking
func UpdateHealthState(memoryID string, isHealthy bool, consecutiveFailures int) {
	state, _ := MemoryHealthStates.LoadOrStore(memoryID, &HealthState{
		MemoryID:            memoryID,
		IsHealthy:           isHealthy,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: consecutiveFailures,
	})
	if healthState, ok := state.(*HealthState); ok {
		healthState.Mu.Lock()
		healthState.IsHealthy = isHealthy
		healthState.LastHealthCheck = time.Now()
		healthState.ConsecutiveFailures = consecutiveFailures
		healthState.Mu.Unlock()
	}
}
