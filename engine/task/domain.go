package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
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
	ExecutionWait       ExecutionType = "wait"
	ExecutionSignal     ExecutionType = "signal"
	ExecutionAggregate  ExecutionType = "aggregate"
	ExecutionMemory     ExecutionType = "memory"
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

	// Aggregated usage information keyed by provider/model.
	Usage *usage.Summary `json:"usage,omitempty" db:"usage"`

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
	UsageRaw       []byte             `db:"usage"`
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
	if len(sdb.UsageRaw) > 0 {
		var summary usage.Summary
		if err := json.Unmarshal(sdb.UsageRaw, &summary); err != nil {
			return nil, fmt.Errorf("unmarshaling usage: %w", err)
		}
		if err := summary.Validate(); err != nil {
			return nil, fmt.Errorf("validating usage: %w", err)
		}
		summary.Sort()
		state.Usage = &summary
	}
	if sdb.ParentStateID.Valid {
		parentID := core.ID(sdb.ParentStateID.String)
		state.ParentStateID = &parentID
	}
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

// CanHaveChildren returns true if this task can have child tasks (parallel, collection, composite, aggregate, memory)
func (s *State) CanHaveChildren() bool {
	return s.ExecutionType == ExecutionParallel ||
		s.ExecutionType == ExecutionCollection ||
		s.ExecutionType == ExecutionComposite ||
		s.ExecutionType == ExecutionAggregate ||
		s.ExecutionType == ExecutionMemory
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
	result.Component = core.ComponentTask
	isValidParentType := result.ExecutionType == ExecutionParallel ||
		result.ExecutionType == ExecutionCollection ||
		result.ExecutionType == ExecutionComposite
	if !isValidParentType {
		result.ExecutionType = ExecutionParallel
	}
	return CreateState(input, result)
}

// -----------------------------------------------------------------------------
// Wait Task Types
// -----------------------------------------------------------------------------

// SignalProcessor handles signal processing logic
type SignalProcessor interface {
	Process(ctx context.Context, signal *SignalEnvelope) (*ProcessorOutput, error)
}

// WaitTaskExecutor defines the main execution interface
type WaitTaskExecutor interface {
	Execute(ctx context.Context, config *Config) (*WaitTaskResult, error)
}

// SignalEnvelope contains signal data and metadata
type SignalEnvelope struct {
	Payload  map[string]any `json:"payload"`  // User-provided data
	Metadata SignalMetadata `json:"metadata"` // System-generated
}

// SignalMetadata provides system-level signal information
type SignalMetadata struct {
	SignalID      string    `json:"signal_id"`       // UUID for deduplication
	ReceivedAtUTC time.Time `json:"received_at_utc"` // Server timestamp
	WorkflowID    string    `json:"workflow_id"`     // Target workflow
	Source        string    `json:"source"`          // Signal source
}

// ProcessorOutput contains processor task results
type ProcessorOutput struct {
	Output any         `json:"output"`
	Error  *core.Error `json:"error,omitempty"`
}

// WaitTaskResult contains workflow output
type WaitTaskResult struct {
	Status          string           `json:"status"`
	Signal          *SignalEnvelope  `json:"signal,omitempty"`
	ProcessorOutput *ProcessorOutput `json:"processor_output,omitempty"`
	NextTask        string           `json:"next_task,omitempty"`
	CompletedAt     time.Time        `json:"completed_at"`
}

// SignalProcessingResult contains activity output
type SignalProcessingResult struct {
	ShouldContinue  bool             `json:"should_continue"`
	Signal          *SignalEnvelope  `json:"signal"`
	ProcessorOutput *ProcessorOutput `json:"processor_output"`
	Reason          string           `json:"reason"`
}
