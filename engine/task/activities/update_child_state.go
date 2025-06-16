package activities

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const UpdateChildStateLabel = "UpdateChildState"

type UpdateChildStateInput struct {
	TaskExecID string       `json:"task_exec_id"`
	Status     string       `json:"status"`
	Output     *core.Output `json:"output,omitempty"`
}

type UpdateChildState struct {
	taskRepo task.Repository
}

// NewUpdateChildState creates a new UpdateChildState activity
func NewUpdateChildState(taskRepo task.Repository) *UpdateChildState {
	return &UpdateChildState{
		taskRepo: taskRepo,
	}
}

func (a *UpdateChildState) Run(ctx context.Context, input map[string]any) error {
	// Extract values from the input map
	taskExecIDStr, ok := input["task_exec_id"].(string)
	if !ok {
		return core.NewError(nil, "INVALID_INPUT", map[string]any{"field": "task_exec_id"})
	}

	statusStr, ok := input["status"].(string)
	if !ok {
		return core.NewError(nil, "INVALID_INPUT", map[string]any{"field": "status"})
	}

	// Convert string to ID
	taskExecID := core.ID(taskExecIDStr)

	// Get the current state
	currentState, err := a.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return core.NewError(err, "LOAD_STATE_FAILED", map[string]any{"task_exec_id": taskExecID})
	}

	// Update the status
	currentState.Status = core.StatusType(statusStr)

	// Update output if provided
	if output, exists := input["output"]; exists && output != nil {
		if outputMap, ok := output.(*core.Output); ok {
			currentState.Output = outputMap
		}
	}

	// Save the updated state
	err = a.taskRepo.UpsertState(ctx, currentState)
	if err != nil {
		return core.NewError(err, "UPDATE_STATE_FAILED", map[string]any{"task_exec_id": taskExecID})
	}

	return nil
}
