package task

import (
	"context"

	"github.com/compozy/compozy/engine/core"
)

type StateFilter struct {
	Status         *core.StatusType `json:"status,omitempty"`
	WorkflowID     *string          `json:"workflow_id,omitempty"`
	WorkflowExecID *core.ID         `json:"workflow_exec_id,omitempty"`
	TaskID         *string          `json:"task_id,omitempty"`
	TaskExecID     *core.ID         `json:"task_exec_id,omitempty"`
	AgentID        *string          `json:"agent_id,omitempty"`
	ToolID         *string          `json:"tool_id,omitempty"`
}

type Repository interface {
	ListStates(ctx context.Context, filter *StateFilter) ([]*State, error)
	UpsertState(ctx context.Context, workflowID string, workflowExecID core.ID, state *State) error
	GetState(ctx context.Context, workflowID string, workflowExecID core.ID, taskStateID StateID) (*State, error)
	GetTaskByID(ctx context.Context, workflowID string, workflowExecID core.ID, taskID string) (*State, error)
	GetTaskByExecID(ctx context.Context, workflowID string, workflowExecID core.ID, taskExecID core.ID) (*State, error)
	GetTaskByAgentID(ctx context.Context, workflowID string, workflowExecID core.ID, agentID string) (*State, error)
	GetTaskByToolID(ctx context.Context, workflowID string, workflowExecID core.ID, toolID string) (*State, error)
	ListTasksInWorkflow(ctx context.Context, workflowExecID core.ID) (map[string]*State, error)
	ListTasksByStatus(ctx context.Context, workflowExecID core.ID, status core.StatusType) ([]*State, error)
	ListTasksByAgent(ctx context.Context, workflowExecID core.ID, agentID string) ([]*State, error)
	ListTasksByTool(ctx context.Context, workflowExecID core.ID, toolID string) ([]*State, error)
}
