package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
)

// InMemoryRepo is an in-memory implementation of task.Repository for testing
type InMemoryRepo struct {
	mu     sync.RWMutex
	states map[core.ID]*task.State
}

// NewInMemoryRepo creates a new in-memory repository
func NewInMemoryRepo() *InMemoryRepo {
	return &InMemoryRepo{
		states: make(map[core.ID]*task.State),
	}
}

// AddState adds a state to the repository
func (r *InMemoryRepo) AddState(state *task.State) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[state.TaskExecID] = state
}

// GetState retrieves a state by task execution ID
func (r *InMemoryRepo) GetState(_ context.Context, taskExecID core.ID) (*task.State, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, exists := r.states[taskExecID]
	if !exists {
		return nil, fmt.Errorf("state not found for task exec ID: %s", taskExecID)
	}

	// Return a copy to prevent race conditions
	stateCopy := *state
	return &stateCopy, nil
}

// UpsertState updates or inserts a state
func (r *InMemoryRepo) UpsertState(_ context.Context, state *task.State) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store a copy to prevent external modifications
	stateCopy := *state
	// Update the timestamp to simulate database behavior
	stateCopy.UpdatedAt = time.Now()
	r.states[state.TaskExecID] = &stateCopy
	return nil
}

// GetStateForUpdate retrieves a state with a lock (for testing, same as GetState)
func (r *InMemoryRepo) GetStateForUpdate(ctx context.Context, _ pgx.Tx, taskExecID core.ID) (*task.State, error) {
	return r.GetState(ctx, taskExecID)
}

// UpsertStateWithTx updates state within a transaction (for testing, same as UpsertState)
func (r *InMemoryRepo) UpsertStateWithTx(ctx context.Context, _ pgx.Tx, state *task.State) error {
	return r.UpsertState(ctx, state)
}

// WithTx executes a function within a transaction (no-op for testing)
func (r *InMemoryRepo) WithTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(nil) // Pass nil transaction for testing
}

// ListChildren retrieves all child states for a given parent
func (r *InMemoryRepo) ListChildren(_ context.Context, parentStateID core.ID) ([]*task.State, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var children []*task.State
	for _, state := range r.states {
		if state.ParentStateID != nil && *state.ParentStateID == parentStateID {
			// Return a copy to prevent race conditions
			stateCopy := *state
			children = append(children, &stateCopy)
		}
	}

	return children, nil
}

// GetChildByTaskID retrieves a specific child task state by its parent and task ID
func (r *InMemoryRepo) GetChildByTaskID(_ context.Context, parentStateID core.ID, taskID string) (*task.State, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, state := range r.states {
		if state.ParentStateID != nil && *state.ParentStateID == parentStateID && state.TaskID == taskID {
			// Return a copy to prevent race conditions
			stateCopy := *state
			return &stateCopy, nil
		}
	}

	return nil, fmt.Errorf("child state not found for parent %s and task %s", parentStateID, taskID)
}

// GetProgressInfo calculates progress information for a parent task
func (r *InMemoryRepo) GetProgressInfo(ctx context.Context, parentStateID core.ID) (*task.ProgressInfo, error) {
	children, err := r.ListChildren(ctx, parentStateID)
	if err != nil {
		return nil, err
	}

	progressInfo := &task.ProgressInfo{
		TotalChildren: len(children),
		StatusCounts:  make(map[core.StatusType]int),
	}

	for _, child := range children {
		progressInfo.StatusCounts[child.Status]++

		switch child.Status {
		case core.StatusSuccess:
			progressInfo.CompletedCount++
		case core.StatusFailed:
			progressInfo.FailedCount++
		case core.StatusRunning:
			progressInfo.RunningCount++
		case core.StatusPending:
			progressInfo.PendingCount++
		}
	}

	// Calculate rates
	if progressInfo.TotalChildren > 0 {
		progressInfo.CompletionRate = float64(progressInfo.CompletedCount) / float64(progressInfo.TotalChildren)
		progressInfo.FailureRate = float64(progressInfo.FailedCount) / float64(progressInfo.TotalChildren)
	}

	return progressInfo, nil
}

// Stub implementations for other interface methods (not used by ParentStatusUpdater)

func (r *InMemoryRepo) ListStates(_ context.Context, _ *task.StateFilter) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) ListTasksInWorkflow(
	_ context.Context,
	_ core.ID,
) (map[string]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) ListTasksByStatus(
	_ context.Context,
	_ core.ID,
	_ core.StatusType,
) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) ListTasksByAgent(
	_ context.Context,
	_ core.ID,
	_ string,
) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) ListTasksByTool(
	_ context.Context,
	_ core.ID,
	_ string,
) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) CreateChildStatesInTransaction(
	_ context.Context,
	_ core.ID,
	_ []*task.State,
) error {
	return fmt.Errorf("not implemented")
}

func (r *InMemoryRepo) GetTaskTree(_ context.Context, _ core.ID) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}
