package task

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

type StateFilter struct {
	Status         *core.StatusType `json:"status,omitempty"`
	WorkflowID     *string          `json:"workflow_id,omitempty"`
	WorkflowExecID *core.ID         `json:"workflow_exec_id,omitempty"`
	TaskID         *string          `json:"task_id,omitempty"`
	TaskExecID     *core.ID         `json:"task_exec_id,omitempty"`
	AgentID        *string          `json:"agent_id,omitempty"`
	ActionID       *string          `json:"action_id,omitempty"`
	ToolID         *string          `json:"tool_id,omitempty"`
}

type Repository interface {
	ListStates(ctx context.Context, filter *StateFilter) ([]*State, error)
	UpsertState(ctx context.Context, state *State) error
	GetState(ctx context.Context, taskExecID core.ID) (*State, error)
	ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*State, error)
	ListTasksByStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) ([]*State, error)
	ListTasksByAgent(ctx context.Context, workflowExecID core.ID, agentID string) ([]*State, error)
	ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*State, error)
}

// -----------------------------------------------------------------------------
// Initial State
// -----------------------------------------------------------------------------

type PartialState struct {
	Component core.ComponentType `json:"component"`
	AgentID   *string            `json:"agent_id,omitempty"`
	ActionID  *string            `json:"action_id,omitempty"`
	ToolID    *string            `json:"tool_id,omitempty"`
	Input     *core.Input        `json:"input,omitempty"`
	MergedEnv core.EnvMap        `json:"merged_env"`
}

type StateInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
	TaskExecID     core.ID `json:"task_exec_id"`
}

func CreateAndPersistState(
	ctx context.Context,
	repo Repository,
	input *StateInput,
	result *PartialState,
) (*State, error) {
	state := &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      result.Component,
		Status:         core.StatusRunning,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		AgentID:        result.AgentID,
		ActionID:       result.ActionID,
		ToolID:         result.ToolID,
		Input:          result.Input,
		Output:         nil,
		Error:          nil,
	}
	if err := repo.UpsertState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to upsert task state: %w", err)
	}
	return state, nil
}
