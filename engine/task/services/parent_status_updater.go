package services

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
)

// ParentStatusUpdater handles updating parent task status based on child task progress
type ParentStatusUpdater struct {
	taskRepo task.Repository
}

// NewParentStatusUpdater creates a new ParentStatusUpdater service
func NewParentStatusUpdater(taskRepo task.Repository) *ParentStatusUpdater {
	return &ParentStatusUpdater{
		taskRepo: taskRepo,
	}
}

// UpdateParentStatusInput contains the parameters for updating parent status
type UpdateParentStatusInput struct {
	ParentStateID core.ID
	Strategy      task.ParallelStrategy
	Recursive     bool
	ChildState    *task.State // Optional: used for recursive updates and error metadata

	// Cycle detection and depth protection
	visited map[core.ID]bool // Track visited parent IDs to prevent cycles
	depth   int              // Current recursion depth
}

const (
	// MaxRecursionDepth limits how deep parent status updates can recurse
	MaxRecursionDepth = 100
)

// validateRecursionSafety checks for cycles and depth limits
func (s *ParentStatusUpdater) validateRecursionSafety(input *UpdateParentStatusInput) error {
	// Initialize visited map if not provided (first call)
	if input.visited == nil {
		input.visited = make(map[core.ID]bool)
	}

	// Check for cycle detection
	if input.visited[input.ParentStateID] {
		return fmt.Errorf("cycle detected in parent state chain at ID %s", input.ParentStateID)
	}

	// Check recursion depth
	if input.depth >= MaxRecursionDepth {
		return fmt.Errorf(
			"maximum recursion depth (%d) exceeded for parent state %s",
			MaxRecursionDepth,
			input.ParentStateID,
		)
	}

	// Mark current ID as visited
	input.visited[input.ParentStateID] = true
	return nil
}

// buildProgressOutput creates the progress metadata output
func (s *ParentStatusUpdater) buildProgressOutput(
	progressInfo *task.ProgressInfo,
	input *UpdateParentStatusInput,
) map[string]any {
	progressOutput := map[string]any{
		"completion_rate": progressInfo.CompletionRate,
		"failure_rate":    progressInfo.FailureRate,
		"total_children":  progressInfo.TotalChildren,
		"completed_count": progressInfo.CompletedCount,
		"failed_count":    progressInfo.FailedCount,
		"running_count":   progressInfo.RunningCount,
		"pending_count":   progressInfo.PendingCount,
		"strategy":        string(input.Strategy),
	}

	// Add last_updated timestamp for handle_resp.go compatibility
	if input.ChildState != nil {
		progressOutput["last_updated"] = time.Now().Format(time.RFC3339)
	}

	return progressOutput
}

// createFailureError creates error metadata when parent task fails
func (s *ParentStatusUpdater) createFailureError(
	progressInfo *task.ProgressInfo,
	input *UpdateParentStatusInput,
) *core.Error {
	errorMetadata := map[string]any{
		"failed_count":    progressInfo.FailedCount,
		"completed_count": progressInfo.CompletedCount,
		"total_children":  progressInfo.TotalChildren,
	}

	// Add child-specific metadata if available
	if input.ChildState != nil {
		errorMetadata["child_task_id"] = input.ChildState.TaskID
		errorMetadata["child_status"] = string(input.ChildState.Status)
	}

	return core.NewError(
		fmt.Errorf("parent task failed due to child task failures"),
		"child_task_failure",
		errorMetadata,
	)
}

// UpdateParentStatus updates a parent task's status based on its children's progress
func (s *ParentStatusUpdater) UpdateParentStatus(
	ctx context.Context,
	input *UpdateParentStatusInput,
) (*task.State, error) {
	var result *task.State

	// Execute within a transaction to ensure atomicity and proper locking
	err := s.taskRepo.WithTx(ctx, func(tx pgx.Tx) error {
		var txErr error
		result, txErr = s.UpdateParentStatusWithTx(ctx, tx, input)
		return txErr
	})

	return result, err
}

// UpdateParentStatusWithTx updates a parent task's status within a transaction with proper locking
func (s *ParentStatusUpdater) UpdateParentStatusWithTx(
	ctx context.Context,
	tx pgx.Tx,
	input *UpdateParentStatusInput,
) (*task.State, error) {
	if err := s.validateRecursionSafety(input); err != nil {
		return nil, err
	}
	defer delete(input.visited, input.ParentStateID) // Clean up on exit

	// Get current parent state with row-level lock to prevent concurrent updates
	parentState, err := s.taskRepo.GetStateForUpdate(ctx, tx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent state %s with lock: %w", input.ParentStateID, err)
	}

	// Get progress information from child tasks
	progressInfo, err := s.taskRepo.GetProgressInfo(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress info for parent %s: %w", input.ParentStateID, err)
	}

	// Calculate new status based on strategy and child progress
	newStatus := progressInfo.CalculateOverallStatus(input.Strategy)

	// Always update progress metadata to keep it fresh
	if parentState.Output == nil {
		output := make(core.Output)
		parentState.Output = &output
	}

	progressOutput := s.buildProgressOutput(progressInfo, input)
	(*parentState.Output)["progress_info"] = progressOutput

	// Track if we need to update the database
	statusChanged := s.updateParentStateStatus(parentState, newStatus, progressInfo, input)

	// Update parent state in database (always update to refresh progress metadata)
	if err := s.taskRepo.UpsertStateWithTx(ctx, tx, parentState); err != nil {
		return nil, fmt.Errorf("failed to update parent state %s: %w", input.ParentStateID, err)
	}

	// Handle recursive updates within the same transaction
	if err := s.handleRecursiveUpdateWithTx(ctx, tx, input, parentState, statusChanged); err != nil {
		return nil, err
	}

	return parentState, nil
}

// updateParentStateStatus updates the parent state's status and error if needed
func (s *ParentStatusUpdater) updateParentStateStatus(
	parentState *task.State,
	newStatus core.StatusType,
	progressInfo *task.ProgressInfo,
	input *UpdateParentStatusInput,
) bool {
	// Only update status if it has changed and task should be updated
	if parentState.Status != newStatus && s.ShouldUpdateParentStatus(parentState.Status, newStatus) {
		parentState.Status = newStatus

		// Set error if parent task failed due to child failures
		if newStatus == core.StatusFailed && progressInfo.HasFailures() {
			parentState.Error = s.createFailureError(progressInfo, input)
		}
		return true
	}
	return false
}

// handleRecursiveUpdateWithTx handles recursive parent status updates within a transaction
func (s *ParentStatusUpdater) handleRecursiveUpdateWithTx(
	ctx context.Context,
	tx pgx.Tx,
	input *UpdateParentStatusInput,
	parentState *task.State,
	statusChanged bool,
) error {
	if !input.Recursive || !statusChanged || parentState.ParentStateID == nil {
		return nil
	}

	_, err := s.UpdateParentStatusWithTx(ctx, tx, &UpdateParentStatusInput{
		ParentStateID: *parentState.ParentStateID,
		Strategy:      input.Strategy,
		Recursive:     true,
		ChildState:    parentState,
		visited:       input.visited,   // Pass visited map to detect cycles
		depth:         input.depth + 1, // Increment depth counter
	})
	if err != nil {
		return fmt.Errorf("failed to recursively update parent status: %w", err)
	}

	return nil
}

// ShouldUpdateParentStatus determines if parent status should be updated
func (s *ParentStatusUpdater) ShouldUpdateParentStatus(currentStatus, newStatus core.StatusType) bool {
	// Don't update if status hasn't changed
	if currentStatus == newStatus {
		return false
	}

	// Allow transitions to terminal states
	if newStatus == core.StatusSuccess || newStatus == core.StatusFailed {
		return true
	}

	// Allow forward-only transitions within active states
	if currentStatus == core.StatusPending && newStatus == core.StatusRunning {
		return true
	}
	if currentStatus == core.StatusRunning && newStatus == core.StatusPending {
		return false
	}
	if currentStatus == core.StatusRunning && newStatus == core.StatusRunning {
		return false
	}

	// Don't update from terminal states unless moving to another terminal state
	if currentStatus == core.StatusSuccess || currentStatus == core.StatusFailed {
		return newStatus == core.StatusSuccess || newStatus == core.StatusFailed
	}

	return false
}
