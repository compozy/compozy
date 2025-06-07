package task

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// StateFilter updated to include execution type filtering
type StateFilter struct {
	Status         *core.StatusType `json:"status,omitempty"`
	WorkflowID     *string          `json:"workflow_id,omitempty"`
	WorkflowExecID *core.ID         `json:"workflow_exec_id,omitempty"`
	TaskID         *string          `json:"task_id,omitempty"`
	TaskExecID     *core.ID         `json:"task_exec_id,omitempty"`
	AgentID        *string          `json:"agent_id,omitempty"`
	ActionID       *string          `json:"action_id,omitempty"`
	ToolID         *string          `json:"tool_id,omitempty"`
	ExecutionType  *ExecutionType   `json:"execution_type,omitempty"`
}

// Repository interface updated with parallel execution methods
type Repository interface {
	// Basic CRUD operations
	ListStates(ctx context.Context, filter *StateFilter) ([]*State, error)
	UpsertState(ctx context.Context, state *State) error
	GetState(ctx context.Context, taskExecID core.ID) (*State, error)

	// Workflow-level operations
	ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*State, error)
	ListTasksByStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) ([]*State, error)
	ListTasksByAgent(ctx context.Context, workflowExecID core.ID, agentID string) ([]*State, error)
	ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*State, error)
}

// -----------------------------------------------------------------------------
// Enhanced State Creation Functions
// -----------------------------------------------------------------------------

// CreateStateInput remains the same
type CreateStateInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
	TaskExecID     core.ID `json:"task_exec_id"`
}

// Enhanced CreateAndPersistState function (already defined in the domain, but including here for completeness)
func CreateAndPersistState(
	ctx context.Context,
	repo Repository,
	input *CreateStateInput,
	result *PartialState,
) (*State, error) {
	var state *State
	switch result.ExecutionType {
	case ExecutionBasic, ExecutionRouter:
		state = CreateBasicState(input, result)
	case ExecutionParallel:
		state = CreateParallelState(input, result)
	default:
		return nil, fmt.Errorf("unsupported execution type: %s", result.ExecutionType)
	}
	if err := repo.UpsertState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to upsert task state: %w", err)
	}
	return state, nil
}
