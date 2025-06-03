package workflow

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
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
	// -----
	// Workflow State Operations
	// -----

	UpsertState(ctx context.Context, state *State) error
	GetState(ctx context.Context, stateID StateID) (*State, error)
	DeleteState(ctx context.Context, stateID StateID) error
	ListStates(ctx context.Context, filter *StateFilter) ([]*State, error)

	// -----
	// Task Management Operations
	// -----

	AddTaskToWorkflow(ctx context.Context, workflowStateID StateID, task *task.State) error
	RemoveTaskFromWorkflow(ctx context.Context, workflowStateID StateID, taskStateID task.StateID) error
	UpdateTaskState(ctx context.Context, workflowStateID StateID, taskStateID task.StateID, task *task.State) error

	// -----
	// Task Query Operations
	// -----

	GetTaskState(ctx context.Context, workflowStateID StateID, taskStateID task.StateID) (*task.State, error)
	GetTaskByID(ctx context.Context, workflowStateID StateID, taskID string) (*task.State, error)
	GetTaskByExecID(ctx context.Context, workflowStateID StateID, taskExecID core.ID) (*task.State, error)
	GetTaskByAgentID(ctx context.Context, workflowStateID StateID, agentID string) (*task.State, error)
	GetTaskByToolID(ctx context.Context, workflowStateID StateID, toolID string) (*task.State, error)

	// -----
	// Task List Operations
	// -----

	ListTasksInWorkflow(ctx context.Context, workflowStateID StateID) (map[string]*task.State, error)
	ListTasksByStatus(ctx context.Context, workflowStateID StateID, status core.StatusType) ([]*task.State, error)
	ListTasksByAgent(ctx context.Context, workflowStateID StateID, agentID string) ([]*task.State, error)
	ListTasksByTool(ctx context.Context, workflowStateID StateID, toolID string) ([]*task.State, error)
}
