package services

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
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
}

// UpdateParentStatus updates a parent task's status based on its children's progress
func (s *ParentStatusUpdater) UpdateParentStatus(
	ctx context.Context,
	input *UpdateParentStatusInput,
) (*task.State, error) {
	// Get current parent state
	parentState, err := s.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent state %s: %w", input.ParentStateID, err)
	}

	// Get progress information from child tasks
	progressInfo, err := s.taskRepo.GetProgressInfo(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress info for parent %s: %w", input.ParentStateID, err)
	}

	// Calculate new status based on strategy and child progress
	newStatus := progressInfo.CalculateOverallStatus(input.Strategy)

	// Only update if status has changed and task should be updated
	if parentState.Status != newStatus && s.ShouldUpdateParentStatus(parentState.Status, newStatus) {
		parentState.Status = newStatus

		// Add progress metadata to parent task output
		if parentState.Output == nil {
			parentState.Output = &core.Output{}
		}

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
			progressOutput["last_updated"] = fmt.Sprintf("%d", time.Now().Unix())
		}

		(*parentState.Output)["progress_info"] = progressOutput

		// Set error if parent task failed due to child failures
		if newStatus == core.StatusFailed && progressInfo.HasFailures() {
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

			parentState.Error = core.NewError(
				fmt.Errorf("parent task failed due to child task failures"),
				"child_task_failure",
				errorMetadata,
			)
		}

		// Update parent state in database
		if err := s.taskRepo.UpsertState(ctx, parentState); err != nil {
			return nil, fmt.Errorf("failed to update parent state %s: %w", input.ParentStateID, err)
		}

		// Recursively update grandparent if needed
		if input.Recursive && parentState.ParentStateID != nil {
			_, err := s.UpdateParentStatus(ctx, &UpdateParentStatusInput{
				ParentStateID: *parentState.ParentStateID,
				Strategy:      input.Strategy,
				Recursive:     true,
				ChildState:    parentState,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to recursively update parent status: %w", err)
			}
		}
	}

	return parentState, nil
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

	// Allow transitions from pending/running to other active states
	if currentStatus == core.StatusPending || currentStatus == core.StatusRunning {
		return true
	}

	// Don't update from terminal states unless moving to another terminal state
	if currentStatus == core.StatusSuccess || currentStatus == core.StatusFailed {
		return newStatus == core.StatusSuccess || newStatus == core.StatusFailed
	}

	return false
}
