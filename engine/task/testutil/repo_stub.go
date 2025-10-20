package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
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

// GetUsageSummary returns a cloned usage summary without loading the full state.
func (r *InMemoryRepo) GetUsageSummary(ctx context.Context, taskExecID core.ID) (*usage.Summary, error) {
	state, err := r.GetState(ctx, taskExecID)
	if err != nil {
		return nil, err
	}
	if state.Usage == nil {
		return nil, nil
	}
	return state.Usage.Clone(), nil
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
func (r *InMemoryRepo) GetStateForUpdate(ctx context.Context, taskExecID core.ID) (*task.State, error) {
	return r.GetState(ctx, taskExecID)
}

// WithTransaction executes a function within a transaction (no-op for testing)
func (r *InMemoryRepo) WithTransaction(_ context.Context, fn func(task.Repository) error) error {
	return fn(r)
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

// ListChildrenOutputs retrieves only the outputs of child tasks for performance
func (r *InMemoryRepo) ListChildrenOutputs(_ context.Context, parentStateID core.ID) (map[string]*core.Output, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	outputs := make(map[string]*core.Output)
	for _, state := range r.states {
		if state.ParentStateID != nil && *state.ParentStateID == parentStateID && state.Output != nil {
			// Return a copy of the output to prevent race conditions
			outputCopy := *state.Output
			outputs[state.TaskID] = &outputCopy
		}
	}
	return outputs, nil
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

func (r *InMemoryRepo) MergeUsage(_ context.Context, taskExecID core.ID, summary *usage.Summary) error {
	if summary == nil || len(summary.Entries) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[taskExecID]
	if !ok {
		return fmt.Errorf("state not found for task exec ID: %s", taskExecID)
	}
	merged := summary.Clone()
	if state.Usage != nil {
		base := state.Usage.Clone()
		if base == nil {
			base = &usage.Summary{}
		}
		base.MergeAll(merged)
		base.Sort()
		state.Usage = base
		return nil
	}
	merged.Sort()
	state.Usage = merged
	return nil
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
			progressInfo.SuccessCount++
		case core.StatusFailed:
			progressInfo.FailedCount++
		case core.StatusCanceled:
			progressInfo.CanceledCount++
		case core.StatusTimedOut:
			progressInfo.TimedOutCount++
		case core.StatusRunning, core.StatusWaiting, core.StatusPaused:
			progressInfo.RunningCount++
		case core.StatusPending:
			progressInfo.PendingCount++
		}
	}
	// Calculate terminal count
	progressInfo.TerminalCount = progressInfo.SuccessCount +
		progressInfo.FailedCount +
		progressInfo.CanceledCount +
		progressInfo.TimedOutCount
	// Calculate rates
	if progressInfo.TotalChildren > 0 {
		progressInfo.CompletionRate = float64(progressInfo.SuccessCount) / float64(progressInfo.TotalChildren)
		progressInfo.FailureRate = float64(
			progressInfo.FailedCount+progressInfo.TimedOutCount,
		) / float64(
			progressInfo.TotalChildren,
		)
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

func (r *InMemoryRepo) GetTaskTree(_ context.Context, _ core.ID) ([]*task.State, error) {
	return nil, fmt.Errorf("not implemented")
}
