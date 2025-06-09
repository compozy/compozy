package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const UpdateParentStatusLabel = "UpdateParentStatus"

type UpdateParentStatusInput struct {
	ParentStateID core.ID               `json:"parent_state_id"`
	Strategy      task.ParallelStrategy `json:"strategy"`
}

type UpdateParentStatus struct {
	taskRepo task.Repository
}

// NewUpdateParentStatus creates a new UpdateParentStatus activity
func NewUpdateParentStatus(taskRepo task.Repository) *UpdateParentStatus {
	return &UpdateParentStatus{
		taskRepo: taskRepo,
	}
}

func (a *UpdateParentStatus) Run(ctx context.Context, input *UpdateParentStatusInput) (*task.State, error) {
	// Get current parent state
	parentState, err := a.taskRepo.GetState(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent state %s: %w", input.ParentStateID, err)
	}

	// Get progress information from child tasks
	progressInfo, err := a.taskRepo.GetProgressInfo(ctx, input.ParentStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get progress info for parent %s: %w", input.ParentStateID, err)
	}

	// Calculate new status based on strategy and child progress
	newStatus := progressInfo.CalculateOverallStatus(input.Strategy)

	// Only update if status has changed and task should be updated
	if parentState.Status != newStatus && a.shouldUpdateParentStatus(parentState.Status, newStatus) {
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
		(*parentState.Output)["progress_info"] = progressOutput

		// Set error if parent task failed due to child failures
		if newStatus == core.StatusFailed && progressInfo.HasFailures() {
			parentState.Error = core.NewError(
				fmt.Errorf("parent task failed due to child task failures"),
				"child_task_failure",
				map[string]any{
					"failed_count":    progressInfo.FailedCount,
					"completed_count": progressInfo.CompletedCount,
					"total_children":  progressInfo.TotalChildren,
				},
			)
		}

		// Update parent state in database
		if err := a.taskRepo.UpsertState(ctx, parentState); err != nil {
			return nil, fmt.Errorf("failed to update parent state %s: %w", input.ParentStateID, err)
		}
	}

	return parentState, nil
}

// shouldUpdateParentStatus determines if parent status should be updated
func (a *UpdateParentStatus) shouldUpdateParentStatus(currentStatus, newStatus core.StatusType) bool {
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
