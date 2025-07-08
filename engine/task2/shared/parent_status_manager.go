package shared

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/segmentio/ksuid"
)

// DefaultParentStatusManager implements ParentStatusManager interface
type DefaultParentStatusManager struct {
	taskRepo    task.Repository
	batchSize   int  // Maximum number of updates to batch together
	enableBatch bool // Enable batch processing for large collections (unexported for encapsulation)
}

// NewParentStatusManager creates a new parent status manager
func NewParentStatusManager(ctx context.Context, taskRepo task.Repository) ParentStatusManager {
	// Read batch size from config or use default
	batchSize := DefaultBatchSize
	// Load configuration from environment
	service := config.NewService()
	appConfig, err := service.Load(ctx)
	if err == nil && appConfig.Limits.ParentUpdateBatchSize > 0 {
		batchSize = appConfig.Limits.ParentUpdateBatchSize
	}

	return &DefaultParentStatusManager{
		taskRepo:    taskRepo,
		batchSize:   batchSize,
		enableBatch: true,
	}
}

// NewParentStatusManagerWithConfig creates a new parent status manager with custom configuration
func NewParentStatusManagerWithConfig(taskRepo task.Repository, batchSize int, enableBatch bool) ParentStatusManager {
	return &DefaultParentStatusManager{
		taskRepo:    taskRepo,
		batchSize:   batchSize,
		enableBatch: enableBatch,
	}
}

// UpdateParentStatus updates parent task status based on child completion
func (m *DefaultParentStatusManager) UpdateParentStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) error {
	// For batch optimization with large collections, delegate to batch method
	if m.enableBatch {
		return m.updateParentStatusBatch(ctx, []ParentUpdate{{
			ParentID: parentStateID,
			Strategy: strategy,
		}})
	}

	return m.updateSingleParentStatus(ctx, parentStateID, strategy)
}

// ParentUpdate represents a parent status update request
type ParentUpdate struct {
	ParentID core.ID
	Strategy task.ParallelStrategy
}

// updateParentStatusBatch efficiently updates multiple parent statuses in batches
func (m *DefaultParentStatusManager) updateParentStatusBatch(
	ctx context.Context,
	updates []ParentUpdate,
) error {
	if len(updates) == 0 {
		return nil
	}

	// Process updates in batches to avoid overwhelming the database
	for i := 0; i < len(updates); i += m.batchSize {
		end := i + m.batchSize
		if end > len(updates) {
			end = len(updates)
		}

		batch := updates[i:end]
		if err := m.processBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// processBatch processes a single batch of parent updates
func (m *DefaultParentStatusManager) processBatch(
	ctx context.Context,
	batch []ParentUpdate,
) error {
	// Build a map to handle duplicates and track strategies
	updateMap := make(map[core.ID]task.ParallelStrategy, len(batch))

	for _, update := range batch {
		if err := m.validateID(update.ParentID); err != nil {
			return fmt.Errorf("invalid parent ID %s: %w", update.ParentID, err)
		}
		// Last strategy wins for duplicate parent IDs
		updateMap[update.ParentID] = update.Strategy
	}

	// Collect unique parent IDs from the map keys
	parentIDs := make([]core.ID, 0, len(updateMap))
	for parentID := range updateMap {
		parentIDs = append(parentIDs, parentID)
	}

	// Batch fetch parent states
	parentStates, err := m.batchGetParentStates(ctx, parentIDs)
	if err != nil {
		return fmt.Errorf("failed to batch get parent states: %w", err)
	}

	// Process each parent's status update
	statesToUpdate := make([]*task.State, 0, len(parentStates))

	for _, parentState := range parentStates {
		strategy := updateMap[parentState.TaskExecID]

		// Get children for this parent
		children, err := m.taskRepo.ListChildren(ctx, parentState.TaskExecID)
		if err != nil {
			return fmt.Errorf("failed to list children for parent %s: %w", parentState.TaskExecID, err)
		}

		if len(children) == 0 {
			continue
		}

		// Calculate aggregated status
		aggregatedStatus := m.calculateStatusWithStrategy(children, strategy)

		// Only update if status changed
		if parentState.Status != aggregatedStatus {
			parentState.Status = aggregatedStatus
			statesToUpdate = append(statesToUpdate, parentState)
		}
	}

	// Batch update all changed states
	if len(statesToUpdate) > 0 {
		if err := m.batchUpdateStates(ctx, statesToUpdate); err != nil {
			return fmt.Errorf("failed to batch update states: %w", err)
		}
	}

	return nil
}

// batchGetParentStates fetches multiple parent states efficiently
func (m *DefaultParentStatusManager) batchGetParentStates(
	ctx context.Context,
	parentIDs []core.ID,
) ([]*task.State, error) {
	// Check if task repository supports batch operations
	type batchRepo interface {
		GetStates(ctx context.Context, ids []core.ID) ([]*task.State, error)
	}

	if br, ok := m.taskRepo.(batchRepo); ok {
		return br.GetStates(ctx, parentIDs)
	}

	// Fallback to individual fetches
	states := make([]*task.State, 0, len(parentIDs))
	for _, id := range parentIDs {
		state, err := m.taskRepo.GetState(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get state %s: %w", id, err)
		}
		states = append(states, state)
	}

	return states, nil
}

// batchUpdateStates updates multiple states efficiently
func (m *DefaultParentStatusManager) batchUpdateStates(
	ctx context.Context,
	states []*task.State,
) error {
	// Check if task repository supports batch operations
	type batchRepo interface {
		UpsertStates(ctx context.Context, states []*task.State) error
	}

	if br, ok := m.taskRepo.(batchRepo); ok {
		return br.UpsertStates(ctx, states)
	}

	// Fallback to individual updates
	for _, state := range states {
		if err := m.taskRepo.UpsertState(ctx, state); err != nil {
			return fmt.Errorf("failed to update state %s: %w", state.TaskExecID, err)
		}
	}

	return nil
}

// updateSingleParentStatus updates a single parent's status
func (m *DefaultParentStatusManager) updateSingleParentStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) error {
	// Validate parent ID to prevent SQL injection
	if err := m.validateID(parentStateID); err != nil {
		return fmt.Errorf("invalid parent task reference: %w", err)
	}
	// Get parent state
	parentState, err := m.taskRepo.GetState(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("unable to retrieve parent task %s: %w", parentStateID, err)
	}
	// Get all children of the parent task
	children, err := m.taskRepo.ListChildren(ctx, parentStateID)
	if err != nil {
		return fmt.Errorf("unable to retrieve child tasks for parent %s: %w", parentStateID, err)
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
			return fmt.Errorf("unable to save parent task status %s: %w", parentStateID, err)
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
	// Validate parent ID to prevent SQL injection
	if err := m.validateID(parentStateID); err != nil {
		return "", fmt.Errorf("invalid parent state ID: %w", err)
	}
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

// validateID validates core.ID to prevent SQL injection
func (m *DefaultParentStatusManager) validateID(id core.ID) error {
	if id == "" {
		return fmt.Errorf("invalid identifier provided")
	}
	// Validate KSUID format
	_, err := ksuid.Parse(id.String())
	if err != nil {
		return fmt.Errorf("invalid ID format: %w", err)
	}
	return nil
}
