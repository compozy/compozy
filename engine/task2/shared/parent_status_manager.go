package shared

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// DefaultParentStatusManager implements ParentStatusManager interface
type DefaultParentStatusManager struct {
	taskRepo task.Repository
}

// NewParentStatusManager creates a new parent status manager
func NewParentStatusManager(taskRepo task.Repository) ParentStatusManager {
	return &DefaultParentStatusManager{
		taskRepo: taskRepo,
	}
}

// UpdateParentStatus updates parent task status based on child completion
func (m *DefaultParentStatusManager) UpdateParentStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) error {
	// Get parent state
	parentState, err := m.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to get parent state %s: %w", parentStateID, err)
	}
	// Get all children of the parent task
	children, err := m.taskRepo.ListChildren(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("failed to list children for parent %s: %w", parentStateID, err)
	}
	if len(children) == 0 {
		return nil
	}
	// Calculate aggregated status using the appropriate strategy
	aggregatedStatus := m.calculateStatusWithStrategy(children, strategy)
	// Update parent status if it has changed
	if parentState.Status != aggregatedStatus {
		parentState.Status = aggregatedStatus
		if err := m.taskRepo.UpsertState(ctx, parentState); err != nil {
			return fmt.Errorf("failed to update parent state %s: %w", parentStateID, err)
		}
	}
	return nil
}

// GetAggregatedStatus calculates the aggregated status for a parent task
func (m *DefaultParentStatusManager) GetAggregatedStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) (core.StatusType, error) {
	children, err := m.taskRepo.ListChildren(ctx, parentStateID)
	if err != nil {
		return "", fmt.Errorf("failed to list children for parent %s: %w", parentStateID, err)
	}
	if len(children) == 0 {
		return core.StatusSuccess, nil
	}
	return m.calculateStatusWithStrategy(children, strategy), nil
}

// calculateStatusWithStrategy calculates status based on specific strategy
func (m *DefaultParentStatusManager) calculateStatusWithStrategy(
	children []*task.State,
	strategy task.ParallelStrategy,
) core.StatusType {
	switch strategy {
	case task.StrategyFailFast:
		return m.calculateFailFastStatus(children)
	case task.StrategyBestEffort:
		return m.calculateBestEffortStatus(children)
	case task.StrategyRace:
		return m.calculateRaceStatus(children)
	case task.StrategyWaitAll, "":
		return m.calculateWaitAllStatus(children)
	default:
		return m.calculateWaitAllStatus(children)
	}
}

// calculateFailFastStatus implements fail-fast strategy
func (m *DefaultParentStatusManager) calculateFailFastStatus(children []*task.State) core.StatusType {
	for _, child := range children {
		if child.Status == core.StatusFailed {
			return core.StatusFailed
		}
		if child.Status == core.StatusRunning || child.Status == core.StatusPending {
			return core.StatusRunning
		}
	}
	return core.StatusSuccess
}

// calculateBestEffortStatus implements best-effort strategy
func (m *DefaultParentStatusManager) calculateBestEffortStatus(children []*task.State) core.StatusType {
	var hasRunning bool
	var hasSuccess bool
	for _, child := range children {
		switch child.Status {
		case core.StatusRunning, core.StatusPending:
			hasRunning = true
		case core.StatusSuccess:
			hasSuccess = true
		}
	}
	if hasRunning {
		return core.StatusRunning
	}
	if hasSuccess {
		return core.StatusSuccess
	}
	return core.StatusFailed
}

// calculateRaceStatus implements race strategy (first to complete wins)
func (m *DefaultParentStatusManager) calculateRaceStatus(children []*task.State) core.StatusType {
	for _, child := range children {
		if child.Status == core.StatusSuccess {
			return core.StatusSuccess
		}
		if child.Status == core.StatusFailed {
			// In race mode, continue to see if others succeed
			continue
		}
	}
	// Check if any are still running
	for _, child := range children {
		if child.Status == core.StatusRunning || child.Status == core.StatusPending {
			return core.StatusRunning
		}
	}
	// All failed
	return core.StatusFailed
}

// calculateWaitAllStatus implements wait-all strategy (default)
func (m *DefaultParentStatusManager) calculateWaitAllStatus(children []*task.State) core.StatusType {
	var successCount, failedCount, runningCount int
	for _, child := range children {
		switch child.Status {
		case core.StatusSuccess:
			successCount++
		case core.StatusFailed:
			failedCount++
		case core.StatusRunning, core.StatusPending:
			runningCount++
		}
	}
	// If any child is running, parent is running
	if runningCount > 0 {
		return core.StatusRunning
	}
	// If any child failed, parent fails
	if failedCount > 0 {
		return core.StatusFailed
	}
	// If all succeeded, parent succeeds
	if successCount == len(children) {
		return core.StatusSuccess
	}
	// Default case
	return core.StatusRunning
}
