package metrics

import (
	"sync"
	"time"
)

// State encapsulates all metric state tracking for distributed memory systems.
// This provides high-performance, thread-safe state management optimized for distributed deployments.
type State struct {
	poolStates   sync.Map // map[string]*PoolState
	tokenStates  sync.Map // map[string]*TokenState
	healthStates sync.Map // map[string]*HealthState
	resetMutex   sync.Mutex
}

// NewState creates a new State instance optimized for distributed systems
func NewState() *State {
	return &State{}
}

// defaultState is the package-level singleton instance
var defaultState *State

// stateOnce ensures the default state is initialized only once
var stateOnce sync.Once

// GetDefaultState returns the singleton State instance
func GetDefaultState() *State {
	stateOnce.Do(func() {
		defaultState = NewState()
	})
	return defaultState
}

// GetPoolState retrieves the pool state for a memory instance
func (s *State) GetPoolState(memoryID string) (*PoolState, bool) {
	state, ok := s.poolStates.Load(memoryID)
	if !ok {
		return nil, false
	}
	poolState, ok := state.(*PoolState)
	if !ok {
		return nil, false
	}
	return poolState, true
}

// GetTokenState retrieves the token state for a memory instance
func (s *State) GetTokenState(memoryID string) (*TokenState, bool) {
	state, ok := s.tokenStates.Load(memoryID)
	if !ok {
		return nil, false
	}
	tokenState, ok := state.(*TokenState)
	if !ok {
		return nil, false
	}
	return tokenState, true
}

// GetHealthState retrieves the health state for a memory instance
func (s *State) GetHealthState(memoryID string) (*HealthState, bool) {
	state, ok := s.healthStates.Load(memoryID)
	if !ok {
		return nil, false
	}
	healthState, ok := state.(*HealthState)
	if !ok {
		return nil, false
	}
	return healthState, true
}

// UpdatePoolState updates the goroutine pool state for a memory instance
func (s *State) UpdatePoolState(memoryID string, activeCount int64, maxPoolSize int64) {
	state, _ := s.poolStates.LoadOrStore(memoryID, &PoolState{
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

// UpdateTokenState updates the token usage state for a memory instance
func (s *State) UpdateTokenState(memoryID string, tokensUsed int64, maxTokens int64) {
	state, _ := s.tokenStates.LoadOrStore(memoryID, &TokenState{
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

// UpdateHealthState updates the health state for a memory instance
func (s *State) UpdateHealthState(memoryID string, isHealthy bool, consecutiveFailures int) {
	state, _ := s.healthStates.LoadOrStore(memoryID, &HealthState{
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

// Clear removes all state data
func (s *State) Clear() {
	s.resetMutex.Lock()
	defer s.resetMutex.Unlock()
	s.poolStates = sync.Map{}
	s.tokenStates = sync.Map{}
	s.healthStates = sync.Map{}
}

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

// RangePoolStates iterates over all pool states
func (s *State) RangePoolStates(f func(key, value any) bool) {
	s.poolStates.Range(f)
}

// RangeTokenStates iterates over all token states
func (s *State) RangeTokenStates(f func(key, value any) bool) {
	s.tokenStates.Range(f)
}

// RangeHealthStates iterates over all health states
func (s *State) RangeHealthStates(f func(key, value any) bool) {
	s.healthStates.Range(f)
}

// LoadPoolState loads a pool state by memory ID
func (s *State) LoadPoolState(memoryID string) (any, bool) {
	return s.poolStates.Load(memoryID)
}

// LoadTokenState loads a token state by memory ID
func (s *State) LoadTokenState(memoryID string) (any, bool) {
	return s.tokenStates.Load(memoryID)
}

// LoadHealthState loads a health state by memory ID
func (s *State) LoadHealthState(memoryID string) (any, bool) {
	return s.healthStates.Load(memoryID)
}

// StoreHealthState stores a health state
func (s *State) StoreHealthState(memoryID string, state *HealthState) {
	s.healthStates.Store(memoryID, state)
}

// DeleteHealthState deletes a health state
func (s *State) DeleteHealthState(memoryID string) {
	s.healthStates.Delete(memoryID)
}
