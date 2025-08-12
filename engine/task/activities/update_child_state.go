package activities

import (
	"context"
	"fmt"

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
		// Try different type assertions with deep copy to avoid aliasing
		switch v := output.(type) {
		case *core.Output:
			if v != nil {
				copied, err := core.DeepCopy[core.Output](*v)
				if err != nil {
					return core.NewError(err, "INVALID_INPUT", map[string]any{"field": "output", "reason": "deepcopy_failed"})
				}
				currentState.Output = &copied
			}
		case core.Output:
			copied, err := core.DeepCopy[core.Output](v)
			if err != nil {
				return core.NewError(err, "INVALID_INPUT", map[string]any{"field": "output", "reason": "deepcopy_failed"})
			}
			currentState.Output = &copied
		case map[string]any:
			copied, err := core.DeepCopy[core.Output](core.Output(v))
			if err != nil {
				return core.NewError(err, "INVALID_INPUT", map[string]any{"field": "output", "reason": "deepcopy_failed"})
			}
			currentState.Output = &copied
		default:
			return core.NewError(nil, "INVALID_INPUT", map[string]any{
				"field":  "output",
				"reason": "unsupported_type",
				"type":   fmt.Sprintf("%T", output),
			})
		}
	}

	// Save the updated state
	err = a.taskRepo.UpsertState(ctx, currentState)
	if err != nil {
		return core.NewError(err, "UPDATE_STATE_FAILED", map[string]any{"task_exec_id": taskExecID})
	}

	return nil
}
