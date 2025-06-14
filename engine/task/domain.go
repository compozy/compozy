package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// Execution Types - New enum to distinguish execution patterns
// -----------------------------------------------------------------------------

type ExecutionType string

const (
	ExecutionBasic      ExecutionType = "basic"
	ExecutionRouter     ExecutionType = "router"
	ExecutionParallel   ExecutionType = "parallel"
	ExecutionCollection ExecutionType = "collection"
	ExecutionComposite  ExecutionType = "composite"
)

// -----------------------------------------------------------------------------
// Enhanced State - Updated to support both basic and parallel execution
// -----------------------------------------------------------------------------

type State struct {
	// Core identification
	Component      core.ComponentType `json:"component"        db:"component"`
	Status         core.StatusType    `json:"status"           db:"status"`
	TaskID         string             `json:"task_id"          db:"task_id"`
	TaskExecID     core.ID            `json:"task_exec_id"     db:"task_exec_id"`
	WorkflowID     string             `json:"workflow_id"      db:"workflow_id"`
	WorkflowExecID core.ID            `json:"workflow_exec_id" db:"workflow_exec_id"`

	// Parent-child relationship for hierarchical tasks
	ParentStateID *core.ID `json:"parent_state_id,omitempty" db:"parent_state_id"`

	// Execution type and strategy
	ExecutionType ExecutionType `json:"execution_type" db:"execution_type"`

	// Basic execution fields (for single tasks)
	AgentID  *string      `json:"agent_id,omitempty"  db:"agent_id"`
	ActionID *string      `json:"action_id,omitempty" db:"action_id"`
	ToolID   *string      `json:"tool_id,omitempty"   db:"tool_id"`
	Input    *core.Input  `json:"input,omitempty"     db:"input"`
	Output   *core.Output `json:"output,omitempty"    db:"output"`
	Error    *core.Error  `json:"error,omitempty"     db:"error"`

	// Timestamps for audit trails and progress tracking
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// -----------------------------------------------------------------------------
// Enhanced StateDB for database operations
// -----------------------------------------------------------------------------

type StateDB struct {
	Component      core.ComponentType `db:"component"`
	Status         core.StatusType    `db:"status"`
	TaskID         string             `db:"task_id"`
	TaskExecID     core.ID            `db:"task_exec_id"`
	WorkflowID     string             `db:"workflow_id"`
	WorkflowExecID core.ID            `db:"workflow_exec_id"`
	ParentStateID  sql.NullString     `db:"parent_state_id"`
	ExecutionType  ExecutionType      `db:"execution_type"`
	AgentIDRaw     sql.NullString     `db:"agent_id"`
	ActionIDRaw    sql.NullString     `db:"action_id"`
	ToolIDRaw      sql.NullString     `db:"tool_id"`
	InputRaw       []byte             `db:"input"`
	OutputRaw      []byte             `db:"output"`
	ErrorRaw       []byte             `db:"error"`
	CreatedAt      time.Time          `db:"created_at"`
	UpdatedAt      time.Time          `db:"updated_at"`
}

// ToState converts StateDB to State with proper JSON unmarshaling
func (sdb *StateDB) ToState() (*State, error) {
	state := &State{
		TaskID:         sdb.TaskID,
		TaskExecID:     sdb.TaskExecID,
		WorkflowID:     sdb.WorkflowID,
		WorkflowExecID: sdb.WorkflowExecID,
		Status:         sdb.Status,
		Component:      sdb.Component,
		ExecutionType:  sdb.ExecutionType,
		CreatedAt:      sdb.CreatedAt,
		UpdatedAt:      sdb.UpdatedAt,
	}

	// Handle parent-child relationship
	if sdb.ParentStateID.Valid {
		parentID := core.ID(sdb.ParentStateID.String)
		state.ParentStateID = &parentID
	}

	// Convert basic fields for all execution types
	err := convertBasic(sdb, state)
	if err != nil {
		return nil, err
	}

	return state, nil
}

func convertBasic(sdb *StateDB, state *State) error {
	if sdb.AgentIDRaw.Valid {
		agentID := sdb.AgentIDRaw.String
		state.AgentID = &agentID
		state.Component = core.ComponentAgent
		if sdb.ActionIDRaw.Valid {
			state.ActionID = &sdb.ActionIDRaw.String
		} else {
			return fmt.Errorf("action_id is required for agent")
		}
	}
	if sdb.ToolIDRaw.Valid {
		state.ToolID = &sdb.ToolIDRaw.String
		state.Component = core.ComponentTool
	}
	if sdb.InputRaw != nil {
		var input core.Input
		if err := json.Unmarshal(sdb.InputRaw, &input); err != nil {
			return fmt.Errorf("unmarshaling input: %w", err)
		}
		state.Input = &input
	}
	if sdb.OutputRaw != nil {
		var output core.Output
		if err := json.Unmarshal(sdb.OutputRaw, &output); err != nil {
			return fmt.Errorf("unmarshaling output: %w", err)
		}
		state.Output = &output
	}
	if sdb.ErrorRaw != nil {
		var errorObj core.Error
		if err := json.Unmarshal(sdb.ErrorRaw, &errorObj); err != nil {
			return fmt.Errorf("unmarshaling error: %w", err)
		}
		state.Error = &errorObj
	}
	return nil
}

// -----------------------------------------------------------------------------
// State methods for hierarchical task management
// -----------------------------------------------------------------------------

// IsParallelExecution returns true if this task has parallel execution type (can have child tasks)
func (s *State) IsParallelExecution() bool {
	return s.ExecutionType == ExecutionParallel
}

// CanHaveChildren returns true if this task can have child tasks (parallel, collection, or composite)
func (s *State) CanHaveChildren() bool {
	return s.ExecutionType == ExecutionParallel ||
		s.ExecutionType == ExecutionCollection ||
		s.ExecutionType == ExecutionComposite
}

// IsChildTask returns true if this task is a child task (has a parent)
func (s *State) IsChildTask() bool {
	return s.ParentStateID != nil
}

// IsBasic returns true if this is a basic execution
func (s *State) IsBasic() bool {
	return s.ExecutionType == ExecutionBasic
}

// IsParallelRoot returns true if this is a parallel root task
// (has ExecutionParallel type and no parent, meaning it's the top-level parallel task)
func (s *State) IsParallelRoot() bool {
	return s.ParentStateID == nil && s.ExecutionType == ExecutionParallel
}

// HasParent returns true if this task has a parent (same as IsChildTask)
func (s *State) HasParent() bool {
	return s.IsChildTask()
}

// GetParentID returns the parent state ID if this task has a parent
func (s *State) GetParentID() *core.ID {
	return s.ParentStateID
}

// ValidateParentChild ensures no circular references
func (s *State) ValidateParentChild(parentID core.ID) error {
	if s.TaskExecID == parentID {
		return fmt.Errorf("task cannot be its own parent")
	}
	return nil
}

// Rest of the existing methods remain the same...
func (s *State) AsMap() (map[core.ID]any, error) {
	val, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var result map[core.ID]any
	err = json.Unmarshal(val, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *State) UpdateStatus(status core.StatusType) {
	s.Status = status
}

// -----------------------------------------------------------------------------
// Enhanced PartialState for creation
// -----------------------------------------------------------------------------

type PartialState struct {
	Component     core.ComponentType `json:"component"`
	ExecutionType ExecutionType      `json:"execution_type"`
	AgentID       *string            `json:"agent_id,omitempty"`
	ActionID      *string            `json:"action_id,omitempty"`
	ToolID        *string            `json:"tool_id,omitempty"`
	Input         *core.Input        `json:"input,omitempty"`
	MergedEnv     *core.EnvMap       `json:"merged_env"`
	ParentStateID *core.ID           `json:"parent_state_id,omitempty"`
}

// CreateBasicPartialState creates a partial state for basic execution
func CreateBasicPartialState(
	component core.ComponentType,
	input *core.Input,
	env *core.EnvMap,
	executionType ExecutionType,
) *PartialState {
	return &PartialState{
		Component:     component,
		ExecutionType: executionType,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateAgentPartialState creates a partial state for agent execution
func CreateAgentPartialState(
	agentID, actionID string,
	input *core.Input,
	env *core.EnvMap,
	executionType ExecutionType,
) *PartialState {
	return &PartialState{
		Component:     core.ComponentAgent,
		ExecutionType: executionType,
		AgentID:       &agentID,
		ActionID:      &actionID,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateToolPartialState creates a partial state for tool execution
func CreateToolPartialState(
	toolID string,
	input *core.Input,
	env *core.EnvMap,
	executionType ExecutionType,
) *PartialState {
	return &PartialState{
		Component:     core.ComponentTool,
		ExecutionType: executionType,
		ToolID:        &toolID,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateParentPartialState creates a partial state for parent task execution
func CreateParentPartialState(
	input *core.Input,
	env *core.EnvMap,
) *PartialState {
	return CreateParentPartialStateWithExecType(input, env, ExecutionParallel)
}

// CreateParentPartialStateWithExecType creates a partial state for parent task execution with custom execution type
func CreateParentPartialStateWithExecType(
	input *core.Input,
	env *core.EnvMap,
	executionType ExecutionType,
) *PartialState {
	return &PartialState{
		Component:     core.ComponentTask,
		ExecutionType: executionType,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateChildPartialState creates a partial state for child task execution
func CreateChildPartialState(
	component core.ComponentType,
	parentStateID core.ID,
	input *core.Input,
	env *core.EnvMap,
	executionType ExecutionType,
) *PartialState {
	return &PartialState{
		Component:     component,
		ExecutionType: executionType,
		ParentStateID: &parentStateID,
		Input:         input,
		MergedEnv:     env,
	}
}

// CreateSubTaskState creates a new sub-task state using regular State
func CreateSubTaskState(
	taskID string,
	taskExecID core.ID,
	workflowID string,
	workflowExecID core.ID,
	parentStateID *core.ID,
	execType ExecutionType,
	component core.ComponentType,
	input *core.Input,
) *State {
	return &State{
		TaskID:         taskID,
		TaskExecID:     taskExecID,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		ParentStateID:  parentStateID,
		Component:      component,
		Status:         core.StatusPending,
		ExecutionType:  execType,
		Input:          input,
	}
}

// CreateAgentSubTaskState creates a sub-task state for agent execution
func CreateAgentSubTaskState(
	taskID string,
	taskExecID core.ID,
	workflowID string,
	workflowExecID core.ID,
	parentStateID *core.ID,
	agentID, actionID string,
	input *core.Input,
) *State {
	subTask := CreateSubTaskState(
		taskID,
		taskExecID,
		workflowID,
		workflowExecID,
		parentStateID,
		ExecutionBasic,
		core.ComponentAgent,
		input,
	)
	subTask.AgentID = &agentID
	subTask.ActionID = &actionID
	return subTask
}

// CreateToolSubTaskState creates a sub-task state for tool execution
func CreateToolSubTaskState(
	taskID string,
	taskExecID core.ID,
	workflowID string,
	workflowExecID core.ID,
	parentStateID *core.ID,
	toolID string,
	input *core.Input,
) *State {
	subTask := CreateSubTaskState(
		taskID,
		taskExecID,
		workflowID,
		workflowExecID,
		parentStateID,
		ExecutionBasic,
		core.ComponentTool,
		input,
	)
	subTask.ToolID = &toolID
	return subTask
}

// -----------------------------------------------------------------------------
// Factory functions for creating states
// -----------------------------------------------------------------------------

// CreateState creates a state with appropriate component-specific fields
func CreateState(input *CreateStateInput, result *PartialState) *State {
	state := &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      result.Component,
		Status:         core.StatusPending,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		ParentStateID:  result.ParentStateID,
		ExecutionType:  result.ExecutionType,
		Input:          result.Input,
		Output:         nil,
		Error:          nil,
	}

	// Set component-specific fields
	if result.AgentID != nil {
		state.AgentID = result.AgentID
		state.ActionID = result.ActionID
	}
	if result.ToolID != nil {
		state.ToolID = result.ToolID
	}

	return state
}

// CreateBasicState creates a basic execution state (kept for backward compatibility)
func CreateBasicState(input *CreateStateInput, result *PartialState) *State {
	return CreateState(input, result)
}

// CreateParentState creates a parent task state (kept for backward compatibility)
func CreateParentState(input *CreateStateInput, result *PartialState) *State {
	// Ensure parent state has correct component
	result.Component = core.ComponentTask
	// Only set ExecutionParallel if the execution type is not already a valid parent type
	isValidParentType := result.ExecutionType == ExecutionParallel ||
		result.ExecutionType == ExecutionCollection ||
		result.ExecutionType == ExecutionComposite
	if !isValidParentType {
		result.ExecutionType = ExecutionParallel
	}
	return CreateState(input, result)
}
