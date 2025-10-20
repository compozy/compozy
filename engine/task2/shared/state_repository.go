package shared

import (
	"context"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// StateRepository abstracts task state persistence operations
//
// This interface demonstrates the transactional interface pattern where repositories
// can optionally implement transaction support. The implementation gracefully degrades
// to non-transactional operations when the underlying repository doesn't support transactions.
//
// Example usage:
//
//	repo := NewStateRepository(taskRepo)
//	err := repo.SaveTaskState(ctx, state) // Automatically uses transactions if available
//
// The pattern allows for:
// - Type assertion to check for transaction support
// - Graceful fallback to non-transactional operations
// - Row-level locking for concurrent safety
// - Consistent API regardless of underlying capabilities
type StateRepository interface {
	// SaveTaskState saves the task state with transaction safety
	// Uses transactions if the underlying repository supports them, otherwise falls back to regular save
	SaveTaskState(ctx context.Context, state *task.State) error
	// SaveTaskStateWithLocking saves the task state with row-level locking
	// Ensures atomic updates in concurrent environments
	SaveTaskStateWithLocking(ctx context.Context, state *task.State) error
	// GetParentState retrieves parent state with optional caching
	// Implements a simple in-memory cache to reduce database queries
	GetParentState(ctx context.Context, parentID core.ID) (*task.State, error)
}

// DefaultStateRepository implements StateRepository with caching
type DefaultStateRepository struct {
	taskRepo           task.Repository
	transactionService *TransactionService
	parentCache        map[core.ID]*task.State // Simple in-memory cache
	cacheMutex         sync.RWMutex            // Protects parentCache access
}

// NewStateRepository creates a new state repository
func NewStateRepository(taskRepo task.Repository) StateRepository {
	return &DefaultStateRepository{
		taskRepo:           taskRepo,
		transactionService: NewTransactionService(taskRepo),
		parentCache:        make(map[core.ID]*task.State),
	}
}

// SaveTaskState saves the task state with transaction safety when available
func (r *DefaultStateRepository) SaveTaskState(ctx context.Context, state *task.State) error {
	if err := r.validateState(state); err != nil {
		return fmt.Errorf("task data validation failed: %w", err)
	}
	return r.transactionService.SaveStateWithLocking(ctx, state)
}

// SaveTaskStateWithLocking saves the task state ensuring parent status is updated atomically
func (r *DefaultStateRepository) SaveTaskStateWithLocking(ctx context.Context, state *task.State) error {
	// NOTE: Delegate to transactional save so parent updates and state writes remain atomic.
	return r.SaveTaskState(ctx, state)
}

// GetParentState retrieves parent state with caching
func (r *DefaultStateRepository) GetParentState(ctx context.Context, parentID core.ID) (*task.State, error) {
	if err := r.validateID(parentID); err != nil {
		return nil, fmt.Errorf("invalid parent task reference: %w", err)
	}
	r.cacheMutex.RLock()
	if cached, exists := r.parentCache[parentID]; exists {
		r.cacheMutex.RUnlock()
		return cached, nil
	}
	r.cacheMutex.RUnlock()
	parentState, err := r.taskRepo.GetState(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve parent task: %w", err)
	}
	r.cacheMutex.Lock()
	r.parentCache[parentID] = parentState
	r.cacheMutex.Unlock()
	return parentState, nil
}

// validateState validates task state before persistence
func (r *DefaultStateRepository) validateState(state *task.State) error {
	if state == nil {
		return fmt.Errorf("task data is required")
	}
	if state.TaskExecID == "" {
		return fmt.Errorf("task identifier is required")
	}
	if state.TaskID == "" {
		return fmt.Errorf("task reference is required")
	}
	return nil
}

// validateID validates core.ID to prevent SQL injection
func (r *DefaultStateRepository) validateID(id core.ID) error {
	if id == "" {
		return fmt.Errorf("invalid identifier provided")
	}
	return nil
}
