package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

const UpdateStateLabel = "UpdateWorkflowState"

type UpdateStateInput struct {
	WorkflowID     string          `json:"workflow_id"`
	WorkflowExecID core.ID         `json:"workflow_exec_id"`
	Status         core.StatusType `json:"status"`
	Error          *core.Error     `json:"error"`
	Output         *core.Output    `json:"output"`
}

type UpdateState struct {
	workflowRepo workflow.Repository
	taskRepo     task.Repository
}

func NewUpdateState(workflowRepo workflow.Repository, taskRepo task.Repository) *UpdateState {
	return &UpdateState{
		workflowRepo: workflowRepo,
		taskRepo:     taskRepo,
	}
}

func (a *UpdateState) Run(ctx context.Context, input *UpdateStateInput) error {
	workflowExecID := input.WorkflowExecID

	// Update workflow state
	state, err := a.workflowRepo.GetState(ctx, workflowExecID)
	if err != nil {
		return fmt.Errorf("failed to get workflow %s: %w", input.WorkflowExecID, err)
	}

	oldStatus := state.Status
	state.WithStatus(input.Status)
	if input.Error != nil {
		state.WithError(input.Error)
	}
	if input.Output != nil {
		state.WithOutput(input.Output)
	}

	if err := a.workflowRepo.UpsertState(ctx, state); err != nil {
		return fmt.Errorf("failed to update workflow %s: %w", input.WorkflowExecID, err)
	}

	// Cascade state changes to tasks when appropriate
	if err := a.cascadeStateToTasks(ctx, workflowExecID, oldStatus, input.Status); err != nil {
		return fmt.Errorf("failed to cascade state to tasks: %w", err)
	}

	return nil
}

// cascadeStateToTasks updates task states when workflow state changes
func (a *UpdateState) cascadeStateToTasks(
	ctx context.Context,
	workflowExecID core.ID,
	_, newStatus core.StatusType,
) error {
	// Only cascade for certain status transitions
	if !shouldCascadeToTasks(newStatus) {
		return nil
	}

	// Get all tasks in the workflow
	tasks, err := a.taskRepo.ListTasksInWorkflow(ctx, workflowExecID)
	if err != nil {
		return fmt.Errorf("failed to list tasks in workflow: %w", err)
	}

	// Update tasks based on the new workflow status
	for _, taskState := range tasks {
		if shouldUpdateTaskState(taskState.Status, newStatus) {
			updatedStatus := getTaskStatusForWorkflowStatus(taskState.Status, newStatus)
			taskState.UpdateStatus(updatedStatus)

			if err := a.taskRepo.UpsertState(ctx, taskState); err != nil {
				return fmt.Errorf("failed to update task %s state: %w", taskState.TaskID, err)
			}
		}
	}

	return nil
}

// shouldCascadeToTasks determines if workflow status changes should cascade to tasks
func shouldCascadeToTasks(workflowStatus core.StatusType) bool {
	switch workflowStatus {
	case core.StatusPaused, core.StatusCanceled, core.StatusFailed:
		return true
	case core.StatusRunning:
		return true // For resume operations
	default:
		return false
	}
}

// shouldUpdateTaskState determines if a task should be updated based on its current status
func shouldUpdateTaskState(taskStatus, workflowStatus core.StatusType) bool {
	// Don't update tasks that have already completed successfully
	if taskStatus == core.StatusSuccess {
		return false
	}

	switch workflowStatus {
	case core.StatusPaused:
		// Only pause tasks that are currently running or pending
		return taskStatus == core.StatusRunning || taskStatus == core.StatusPending
	case core.StatusRunning:
		// Only resume tasks that are currently paused
		return taskStatus == core.StatusPaused
	case core.StatusCanceled, core.StatusFailed:
		// Cancel/fail all non-completed tasks
		return taskStatus == core.StatusPending || taskStatus == core.StatusRunning || taskStatus == core.StatusPaused
	default:
		return false
	}
}

// getTaskStatusForWorkflowStatus determines the appropriate task status based on workflow status
func getTaskStatusForWorkflowStatus(currentTaskStatus, workflowStatus core.StatusType) core.StatusType {
	switch workflowStatus {
	case core.StatusPaused:
		return core.StatusPaused
	case core.StatusRunning:
		// When resuming, return tasks to their appropriate active state
		if currentTaskStatus == core.StatusPaused {
			return core.StatusPending // Resume to pending, they'll become running when executed
		}
		return currentTaskStatus
	case core.StatusCanceled:
		return core.StatusCanceled
	case core.StatusFailed:
		return core.StatusFailed
	default:
		return currentTaskStatus
	}
}
