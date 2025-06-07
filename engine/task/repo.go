package task

import (
	"context"
	"fmt"
	"time"

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

	// Parallel execution specific operations
	ListParallelTasks(ctx context.Context, workflowExecID core.ID) ([]*State, error)
	UpdateSubTaskStatus(
		ctx context.Context,
		parentTaskExecID core.ID,
		subTaskID string,
		status core.StatusType,
		output *core.Output,
		err *core.Error,
	) error
	GetSubTaskProgress(ctx context.Context, taskExecID core.ID) (completed, failed, total int, err error)
	ListRunningSubTasks(ctx context.Context, parentTaskExecID core.ID) ([]string, error)
	GetParallelTaskBySubTaskID(ctx context.Context, workflowExecID core.ID, subTaskID string) (*State, error)
	BulkUpdateSubTaskStatuses(ctx context.Context, parentTaskExecID core.ID, updates map[string]struct {
		Status core.StatusType
		Output *core.Output
		Error  *core.Error
	}) error
}

// -----------------------------------------------------------------------------
// Enhanced State Creation Functions
// -----------------------------------------------------------------------------

// StateInput remains the same
type StateInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	TaskID         string  `json:"task_id"`
	TaskExecID     core.ID `json:"task_exec_id"`
}

// CreateBasicPartialState creates a partial state for basic execution
func CreateBasicPartialState(component core.ComponentType, input *core.Input, env core.EnvMap) *PartialState {
	return &PartialState{
		Component:     component,
		ExecutionType: ExecutionBasic,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateAgentPartialState creates a partial state for agent execution
func CreateAgentPartialState(agentID, actionID string, input *core.Input, env core.EnvMap) *PartialState {
	return &PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: ExecutionBasic,
		AgentID:       &agentID,
		ActionID:      &actionID,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateToolPartialState creates a partial state for tool execution
func CreateToolPartialState(toolID string, input *core.Input, env core.EnvMap) *PartialState {
	return &PartialState{
		Component:     core.ComponentTool,
		ExecutionType: ExecutionBasic,
		ToolID:        &toolID,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateParallelPartialState creates a partial state for parallel execution
func CreateParallelPartialState(
	strategy ParallelStrategy,
	maxWorkers int,
	timeout string,
	subTasks map[string]*SubTaskState,
	env core.EnvMap,
) *PartialState {
	parallelState := &ParallelExecutionState{
		Strategy:         strategy,
		MaxWorkers:       maxWorkers,
		Timeout:          timeout,
		SubTasks:         subTasks,
		CompletedTasks:   make([]string, 0),
		FailedTasks:      make([]string, 0),
		AggregatedOutput: make(map[string]*core.Output),
	}

	return &PartialState{
		Component:     core.ComponentTask,
		ExecutionType: ExecutionParallel,
		ParallelState: parallelState,
		MergedEnv:     env,
	}
}

// Enhanced CreateAndPersistState function (already defined in the domain, but including here for completeness)
func CreateAndPersistState(
	ctx context.Context,
	repo Repository,
	input *StateInput,
	result *PartialState,
) (*State, error) {
	var state *State

	switch result.ExecutionType {
	case ExecutionBasic:
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

// -----------------------------------------------------------------------------
// Helper functions for working with parallel states
// -----------------------------------------------------------------------------

// CreateSubTaskState creates a new sub-task state
func CreateSubTaskState(
	taskID string,
	taskExecID core.ID,
	component core.ComponentType,
	input *core.Input,
) *SubTaskState {
	now := time.Now()
	return &SubTaskState{
		TaskID:     taskID,
		TaskExecID: taskExecID,
		Component:  component,
		Status:     core.StatusPending,
		Input:      input,
		StartedAt:  &now,
	}
}

// CreateAgentSubTaskState creates a sub-task state for agent execution
func CreateAgentSubTaskState(
	taskID string,
	taskExecID core.ID,
	agentID, actionID string,
	input *core.Input,
) *SubTaskState {
	subTask := CreateSubTaskState(taskID, taskExecID, core.ComponentAgent, input)
	subTask.AgentID = &agentID
	subTask.ActionID = &actionID
	return subTask
}

// CreateToolSubTaskState creates a sub-task state for tool execution
func CreateToolSubTaskState(
	taskID string,
	taskExecID core.ID,
	toolID string,
	input *core.Input,
) *SubTaskState {
	subTask := CreateSubTaskState(taskID, taskExecID, core.ComponentTool, input)
	subTask.ToolID = &toolID
	return subTask
}

// -----------------------------------------------------------------------------
// Utility functions for state filtering
// -----------------------------------------------------------------------------

// NewBasicTaskFilter creates a filter for basic execution tasks
func NewBasicTaskFilter() *StateFilter {
	execType := ExecutionBasic
	return &StateFilter{
		ExecutionType: &execType,
	}
}

// NewParallelTaskFilter creates a filter for parallel execution tasks
func NewParallelTaskFilter() *StateFilter {
	execType := ExecutionParallel
	return &StateFilter{
		ExecutionType: &execType,
	}
}

// NewWorkflowTaskFilter creates a filter for tasks in a specific workflow execution
func NewWorkflowTaskFilter(workflowExecID core.ID) *StateFilter {
	return &StateFilter{
		WorkflowExecID: &workflowExecID,
	}
}

// NewStatusTaskFilter creates a filter for tasks with a specific status
func NewStatusTaskFilter(status core.StatusType) *StateFilter {
	return &StateFilter{
		Status: &status,
	}
}

// CombineFilters combines multiple filters into one (AND operation)
func CombineFilters(filters ...*StateFilter) *StateFilter {
	if len(filters) == 0 {
		return &StateFilter{}
	}

	combined := &StateFilter{}
	for _, filter := range filters {
		if filter == nil {
			continue
		}

		if filter.Status != nil {
			combined.Status = filter.Status
		}
		if filter.WorkflowID != nil {
			combined.WorkflowID = filter.WorkflowID
		}
		if filter.WorkflowExecID != nil {
			combined.WorkflowExecID = filter.WorkflowExecID
		}
		if filter.TaskID != nil {
			combined.TaskID = filter.TaskID
		}
		if filter.TaskExecID != nil {
			combined.TaskExecID = filter.TaskExecID
		}
		if filter.AgentID != nil {
			combined.AgentID = filter.AgentID
		}
		if filter.ActionID != nil {
			combined.ActionID = filter.ActionID
		}
		if filter.ToolID != nil {
			combined.ToolID = filter.ToolID
		}
		if filter.ExecutionType != nil {
			combined.ExecutionType = filter.ExecutionType
		}
	}

	return combined
}
