package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// -----------------------------------------------------------------------------
// Execution Types - New enum to distinguish execution patterns
// -----------------------------------------------------------------------------

type ExecutionType string

const (
	ExecutionBasic    ExecutionType = "basic"
	ExecutionRouter   ExecutionType = "router"
	ExecutionParallel ExecutionType = "parallel"
)

// -----------------------------------------------------------------------------
// Parallel Execution State - Updated to use regular State for sub-tasks
// -----------------------------------------------------------------------------

type ParallelState struct {
	Strategy       ParallelStrategy  `json:"strategy"`
	MaxWorkers     int               `json:"max_workers"`
	Timeout        string            `json:"timeout,omitempty"`
	SubTasks       map[string]*State `json:"sub_tasks"`       // Map of sub-task ID to State
	CompletedTasks []string          `json:"completed_tasks"` // List of completed sub-task IDs
	FailedTasks    []string          `json:"failed_tasks"`    // List of failed sub-task IDs
}

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

	// Execution type and strategy
	ExecutionType ExecutionType `json:"execution_type" db:"execution_type"`

	// Basic execution fields (for single tasks)
	AgentID  *string      `json:"agent_id,omitempty"  db:"agent_id"`
	ActionID *string      `json:"action_id,omitempty" db:"action_id"`
	ToolID   *string      `json:"tool_id,omitempty"   db:"tool_id"`
	Input    *core.Input  `json:"input,omitempty"     db:"input"`
	Output   *core.Output `json:"output,omitempty"    db:"output"`
	Error    *core.Error  `json:"error,omitempty"     db:"error"`

	// Parallel execution fields (embedded inline for JSON, separate column for DB)
	*ParallelState `json:",inline" db:"parallel_state"`
}

// -----------------------------------------------------------------------------
// Enhanced StateDB for database operations
// -----------------------------------------------------------------------------

type StateDB struct {
	Component        core.ComponentType `db:"component"`
	Status           core.StatusType    `db:"status"`
	TaskID           string             `db:"task_id"`
	TaskExecID       core.ID            `db:"task_exec_id"`
	WorkflowID       string             `db:"workflow_id"`
	WorkflowExecID   core.ID            `db:"workflow_exec_id"`
	ExecutionType    ExecutionType      `db:"execution_type"`
	AgentIDRaw       sql.NullString     `db:"agent_id"`
	ActionIDRaw      sql.NullString     `db:"action_id"`
	ToolIDRaw        sql.NullString     `db:"tool_id"`
	InputRaw         []byte             `db:"input"`
	OutputRaw        []byte             `db:"output"`
	ErrorRaw         []byte             `db:"error"`
	ParallelStateRaw []byte             `db:"parallel_state"`
	CreatedAt        time.Time          `db:"created_at"`
	UpdatedAt        time.Time          `db:"updated_at"`
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
	}
	if sdb.ExecutionType == ExecutionBasic || sdb.ExecutionType == ExecutionRouter {
		err := convertBasic(sdb, state)
		if err != nil {
			return nil, err
		}
	}
	if sdb.ExecutionType == ExecutionParallel {
		err := convertParallel(sdb, state)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}

func convertParallel(sdb *StateDB, state *State) error {
	if sdb.ParallelStateRaw != nil {
		var parallelState ParallelState
		if err := json.Unmarshal(sdb.ParallelStateRaw, &parallelState); err != nil {
			return fmt.Errorf("unmarshaling parallel state: %w", err)
		}
		state.ParallelState = &parallelState
	}
	if sdb.InputRaw != nil {
		var input core.Input
		if err := json.Unmarshal(sdb.InputRaw, &input); err != nil {
			return fmt.Errorf("unmarshaling parallel task input: %w", err)
		}
		state.Input = &input
	}
	if sdb.OutputRaw != nil {
		var output core.Output
		if err := json.Unmarshal(sdb.OutputRaw, &output); err != nil {
			return fmt.Errorf("unmarshaling parallel task output: %w", err)
		}
		state.Output = &output
	}
	if sdb.ErrorRaw != nil {
		var errorObj core.Error
		if err := json.Unmarshal(sdb.ErrorRaw, &errorObj); err != nil {
			return fmt.Errorf("unmarshaling parallel task error: %w", err)
		}
		state.Error = &errorObj
	}
	return nil
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
// State methods for parallel execution
// -----------------------------------------------------------------------------

// IsParallel returns true if this is a parallel execution
func (s *State) IsParallel() bool {
	return s.ExecutionType == ExecutionParallel
}

// IsBasic returns true if this is a basic execution
func (s *State) IsBasic() bool {
	return s.ExecutionType == ExecutionBasic
}

// GetSubTaskState returns the state of a specific sub-task
func (s *State) GetSubTaskState(taskID string) (*State, bool) {
	if !s.IsParallel() || s.ParallelState == nil {
		return nil, false
	}
	subTask, exists := s.SubTasks[taskID]
	return subTask, exists
}

// AddSubTask adds a new sub-task to parallel execution
func (s *State) AddSubTask(subTask *State) error {
	if !s.IsParallel() {
		return fmt.Errorf("cannot add sub-task to non-parallel execution")
	}
	if s.ParallelState == nil {
		s.ParallelState = &ParallelState{
			SubTasks:       make(map[string]*State),
			CompletedTasks: make([]string, 0),
			FailedTasks:    make([]string, 0),
		}
	}
	s.SubTasks[subTask.TaskID] = subTask
	return nil
}

// UpdateSubtaskState updates the status of a sub-task
func (s *State) UpdateSubtaskState(
	taskID string,
	status core.StatusType,
	output *core.Output,
	err *core.Error,
) (*State, error) {
	if !s.IsParallel() || s.ParallelState == nil {
		return nil, fmt.Errorf("cannot update sub-task in non-parallel execution")
	}
	subTask, exists := s.SubTasks[taskID]
	if !exists {
		return nil, fmt.Errorf("sub-task %s not found", taskID)
	}
	// Update the sub-task
	subTask.Status = status
	subTask.Output = output
	subTask.Error = err
	switch status {
	case core.StatusSuccess:
		s.CompletedTasks = append(s.CompletedTasks, taskID)
	case core.StatusFailed:
		s.FailedTasks = append(s.FailedTasks, taskID)
	}
	s.updateOverallStatus()
	return subTask, nil
}

// updateOverallStatus determines the overall parallel task status based on strategy
func (s *State) updateOverallStatus() {
	if !s.IsParallel() || s.ParallelState == nil {
		return
	}
	totalTasks := len(s.SubTasks)
	completedCount := len(s.CompletedTasks)
	failedCount := len(s.FailedTasks)
	switch s.Strategy {
	case StrategyWaitAll:
		s.updateStatusForWaitAll(completedCount, failedCount, totalTasks)
	case StrategyFailFast:
		s.updateStatusForFailFast(completedCount, failedCount, totalTasks)
	case StrategyBestEffort:
		s.updateStatusForBestEffort(completedCount, failedCount, totalTasks)
	case StrategyRace:
		s.updateStatusForRace(completedCount, failedCount, totalTasks)
	}
}

func (s *State) updateStatusForWaitAll(completedCount, failedCount, totalTasks int) {
	if failedCount > 0 {
		// For wait_all strategy, any failure should cause the entire parallel execution to fail
		s.Status = core.StatusFailed
	} else if completedCount == totalTasks {
		s.Status = core.StatusSuccess
	}
}

func (s *State) updateStatusForFailFast(completedCount, failedCount, totalTasks int) {
	if failedCount > 0 {
		s.Status = core.StatusFailed
	} else if completedCount == totalTasks {
		s.Status = core.StatusSuccess
	}
}

func (s *State) updateStatusForBestEffort(completedCount, failedCount, totalTasks int) {
	if (completedCount + failedCount) == totalTasks {
		if completedCount > 0 {
			s.Status = core.StatusSuccess
		} else {
			s.Status = core.StatusFailed
		}
	}
}

func (s *State) updateStatusForRace(completedCount, failedCount, totalTasks int) {
	if completedCount > 0 {
		s.Status = core.StatusSuccess
	} else if failedCount == totalTasks {
		s.Status = core.StatusFailed
	}
}

// GetParallelProgress returns progress information for parallel execution
func (s *State) GetParallelProgress() (completed, failed, total int) {
	if !s.IsParallel() || s.ParallelState == nil {
		return 0, 0, 0
	}
	return len(s.CompletedTasks),
		len(s.FailedTasks),
		len(s.SubTasks)
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

func (s *State) IsParallelFailed() error {
	var executionError error
	strategy := s.Strategy
	completed, failed, total := s.GetParallelProgress()

	switch strategy {
	case StrategyWaitAll:
		// wait_all: fail if ANY subtask failed
		if failed > 0 {
			executionError = fmt.Errorf("parallel execution failed: %d out of %d subtasks failed", failed, total)
		}
	case StrategyFailFast:
		// fail_fast: fail if ANY subtask failed
		if failed > 0 {
			executionError = fmt.Errorf("parallel execution failed fast: %d out of %d subtasks failed", failed, total)
		}
	case StrategyBestEffort:
		// best_effort: only fail if ALL subtasks failed
		if failed == total && total > 0 {
			executionError = fmt.Errorf("parallel execution failed: all %d subtasks failed", total)
		}
	case StrategyRace:
		// race: fail if all subtasks failed and none completed
		if failed == total && completed == 0 && total > 0 {
			executionError = fmt.Errorf("parallel execution failed: all %d subtasks failed in race", total)
		}
	}
	return executionError
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
	ParallelState *ParallelState     `json:"parallel_state,omitempty"`
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

// CreateParallelPartialState creates a partial state for parallel execution
func CreateParallelPartialState(
	strategy ParallelStrategy,
	maxWorkers int,
	timeout string,
	subTasks map[string]*State,
	env *core.EnvMap,
) *PartialState {
	return &PartialState{
		Component:     core.ComponentTask,
		ExecutionType: ExecutionParallel,
		MergedEnv:     env,
		ParallelState: &ParallelState{
			Strategy:       strategy,
			MaxWorkers:     maxWorkers,
			Timeout:        timeout,
			SubTasks:       subTasks,
			CompletedTasks: make([]string, 0),
			FailedTasks:    make([]string, 0),
		},
	}
}

// CreateSubTaskState creates a new sub-task state using regular State
func CreateSubTaskState(
	taskID string,
	taskExecID core.ID,
	workflowID string,
	workflowExecID core.ID,
	execType ExecutionType,
	component core.ComponentType,
	input *core.Input,
) *State {
	return &State{
		TaskID:         taskID,
		TaskExecID:     taskExecID,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
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
	agentID, actionID string,
	input *core.Input,
) *State {
	subTask := CreateSubTaskState(
		taskID,
		taskExecID,
		workflowID,
		workflowExecID,
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
	toolID string,
	input *core.Input,
) *State {
	subTask := CreateSubTaskState(
		taskID,
		taskExecID,
		workflowID,
		workflowExecID,
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

// CreateBasicState creates a basic execution state
func CreateBasicState(input *CreateStateInput, result *PartialState) *State {
	return &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      result.Component,
		Status:         core.StatusPending,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		ExecutionType:  result.ExecutionType,
		AgentID:        result.AgentID,
		ActionID:       result.ActionID,
		ToolID:         result.ToolID,
		Input:          result.Input,
		Output:         nil,
		Error:          nil,
	}
}

// CreateParallelState creates a parallel execution state
func CreateParallelState(input *CreateStateInput, result *PartialState) *State {
	return &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      core.ComponentTask,
		Status:         core.StatusRunning,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		ExecutionType:  ExecutionParallel,
		ParallelState:  result.ParallelState,
		Input:          result.Input,
		Output:         nil,
		Error:          nil,
	}
}

// -----------------------------------------------------------------------------
// Response - Enhanced to handle both basic and parallel execution
// -----------------------------------------------------------------------------

type Response struct {
	OnSuccess *core.SuccessTransition `json:"on_success"`
	OnError   *core.ErrorTransition   `json:"on_error"`
	State     *State                  `json:"state"`
	NextTask  *Config                 `json:"next_task"`
}

func (r *Response) NextTaskID() string {
	state := r.State
	taskID := state.TaskID
	var nextTaskID string

	switch {
	case state.Status == core.StatusSuccess && r.OnSuccess != nil && r.OnSuccess.Next != nil:
		nextTaskID = *r.OnSuccess.Next
		logger.Info("Task succeeded, transitioning to next task",
			"current_task", taskID,
			"next_task", nextTaskID,
		)
	case state.Status == core.StatusFailed && r.OnError != nil && r.OnError.Next != nil:
		nextTaskID = *r.OnError.Next
		logger.Info("Task failed, transitioning to error task",
			"current_task", taskID,
			"next_task", nextTaskID,
		)
	default:
		logger.Info("No more tasks to execute", "current_task", taskID)
	}
	return nextTaskID
}

type SubtaskResponse struct {
	TaskID string          `json:"task_id"`
	Output *core.Output    `json:"output"`
	Error  *core.Error     `json:"error"`
	Status core.StatusType `json:"status"`
}
