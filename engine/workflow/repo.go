package workflow

import (
	"context"

	"github.com/compozy/compozy/engine/core"
)

type StateFilter struct {
	Status         *core.StatusType `json:"status,omitempty"`
	WorkflowID     *string          `json:"workflow_id,omitempty"`
	WorkflowExecID *core.ID         `json:"workflow_exec_id,omitempty"`
}

type Repository interface {
	ListStates(ctx context.Context, filter *StateFilter) ([]*State, error)
	UpsertState(ctx context.Context, state *State) error
	UpdateStatus(ctx context.Context, workflowExecID string, status core.StatusType) error
	GetState(ctx context.Context, workflowExecID core.ID) (*State, error)
	GetStateByID(ctx context.Context, workflowID string) (*State, error)
	GetStateByTaskID(ctx context.Context, workflowID, taskID string) (*State, error)
	GetStateByAgentID(ctx context.Context, workflowID, agentID string) (*State, error)
	GetStateByToolID(ctx context.Context, workflowID, toolID string) (*State, error)
}
