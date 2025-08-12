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

// to load and persist child task states.
func NewUpdateChildState(taskRepo task.Repository) *UpdateChildState {
	return &UpdateChildState{
		taskRepo: taskRepo,
	}
}

func (a *UpdateChildState) validateInput(input map[string]any) (core.ID, core.StatusType, error) {
	taskExecIDStr, ok := input["task_exec_id"].(string)
	if !ok {
		return "", "", core.NewError(nil, "INVALID_INPUT", map[string]any{"field": "task_exec_id"})
	}
	statusStr, ok := input["status"].(string)
	if !ok {
		return "", "", core.NewError(nil, "INVALID_INPUT", map[string]any{"field": "status"})
	}
	return core.ID(taskExecIDStr), core.StatusType(statusStr), nil
}

func (a *UpdateChildState) updateOutput(currentState *task.State, input map[string]any) error {
	output, exists := input["output"]
	if !exists {
		return nil
	}
	if output == nil {
		currentState.Output = nil
		return nil
	}
	switch v := output.(type) {
	case *core.Output:
		if v != nil {
			copied, err := core.DeepCopy(*v)
			if err != nil {
				return core.NewError(err, "INVALID_INPUT", map[string]any{"field": "output", "reason": "deepcopy_failed"})
			}
			currentState.Output = &copied
		}
	case core.Output:
		copied, err := core.DeepCopy(v)
		if err != nil {
			return core.NewError(err, "INVALID_INPUT", map[string]any{"field": "output", "reason": "deepcopy_failed"})
		}
		currentState.Output = &copied
	case map[string]any:
		copied, err := core.DeepCopy(core.Output(v))
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
	return nil
}

func (a *UpdateChildState) Run(ctx context.Context, input map[string]any) error {
	taskExecID, status, err := a.validateInput(input)
	if err != nil {
		return err
	}
	currentState, err := a.taskRepo.GetState(ctx, taskExecID)
	if err != nil {
		return core.NewError(err, "LOAD_STATE_FAILED", map[string]any{"task_exec_id": taskExecID})
	}
	currentState.Status = status
	if err := a.updateOutput(currentState, input); err != nil {
		return err
	}
	err = a.taskRepo.UpsertState(ctx, currentState)
	if err != nil {
		return core.NewError(err, "UPDATE_STATE_FAILED", map[string]any{"task_exec_id": taskExecID})
	}
	return nil
}
