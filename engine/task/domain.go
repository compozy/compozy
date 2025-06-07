// Enhanced domain.go with parallel task support

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
	ExecutionBasic    ExecutionType = "basic"    // Single task execution
	ExecutionParallel ExecutionType = "parallel" // Parallel task execution
)

// -----------------------------------------------------------------------------
// Parallel Execution State - New structure for parallel task state
// -----------------------------------------------------------------------------

type ParallelExecutionState struct {
	Strategy         ParallelStrategy         `json:"strategy"`
	MaxWorkers       int                      `json:"max_workers"`
	Timeout          string                   `json:"timeout,omitempty"`
	SubTasks         map[string]*SubTaskState `json:"sub_tasks"`         // Map of sub-task ID to state
	CompletedTasks   []string                 `json:"completed_tasks"`   // List of completed sub-task IDs
	FailedTasks      []string                 `json:"failed_tasks"`      // List of failed sub-task IDs
	AggregatedOutput map[string]*core.Output  `json:"aggregated_output"` // Combined outputs from sub-tasks
}

type SubTaskState struct {
	TaskID      string             `json:"task_id"`
	TaskExecID  core.ID            `json:"task_exec_id"`
	Component   core.ComponentType `json:"component"`
	Status      core.StatusType    `json:"status"`
	AgentID     *string            `json:"agent_id,omitempty"`
	ActionID    *string            `json:"action_id,omitempty"`
	ToolID      *string            `json:"tool_id,omitempty"`
	Input       *core.Input        `json:"input,omitempty"`
	Output      *core.Output       `json:"output,omitempty"`
	Error       *core.Error        `json:"error,omitempty"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
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

	// Parallel execution fields
	ParallelState *ParallelExecutionState `json:"parallel_state,omitempty" db:"parallel_state"`
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

	// Handle basic execution fields
	if sdb.ExecutionType == ExecutionBasic {
		// Handle nullable AgentID
		if sdb.AgentIDRaw.Valid {
			agentID := sdb.AgentIDRaw.String
			state.AgentID = &agentID
			state.Component = core.ComponentAgent

			// Handle nullable ActionID
			if sdb.ActionIDRaw.Valid {
				state.ActionID = &sdb.ActionIDRaw.String
			} else {
				return nil, fmt.Errorf("action_id is required for agent")
			}
		}

		// Handle nullable ToolID
		if sdb.ToolIDRaw.Valid {
			state.ToolID = &sdb.ToolIDRaw.String
			state.Component = core.ComponentTool
		}

		// Unmarshal basic execution input/output/error
		if sdb.InputRaw != nil {
			var input core.Input
			if err := json.Unmarshal(sdb.InputRaw, &input); err != nil {
				return nil, fmt.Errorf("unmarshaling input: %w", err)
			}
			state.Input = &input
		}

		if sdb.OutputRaw != nil {
			var output core.Output
			if err := json.Unmarshal(sdb.OutputRaw, &output); err != nil {
				return nil, fmt.Errorf("unmarshaling output: %w", err)
			}
			state.Output = &output
		}

		if sdb.ErrorRaw != nil {
			var errorObj core.Error
			if err := json.Unmarshal(sdb.ErrorRaw, &errorObj); err != nil {
				return nil, fmt.Errorf("unmarshaling error: %w", err)
			}
			state.Error = &errorObj
		}
	}

	// Handle parallel execution fields
	if sdb.ExecutionType == ExecutionParallel && sdb.ParallelStateRaw != nil {
		var parallelState ParallelExecutionState
		if err := json.Unmarshal(sdb.ParallelStateRaw, &parallelState); err != nil {
			return nil, fmt.Errorf("unmarshaling parallel state: %w", err)
		}
		state.ParallelState = &parallelState
	}

	return state, nil
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
func (s *State) GetSubTaskState(taskID string) (*SubTaskState, bool) {
	if !s.IsParallel() || s.ParallelState == nil {
		return nil, false
	}
	subTask, exists := s.ParallelState.SubTasks[taskID]
	return subTask, exists
}

// AddSubTask adds a new sub-task to parallel execution
func (s *State) AddSubTask(subTask *SubTaskState) error {
	if !s.IsParallel() {
		return fmt.Errorf("cannot add sub-task to non-parallel execution")
	}
	if s.ParallelState == nil {
		s.ParallelState = &ParallelExecutionState{
			SubTasks:         make(map[string]*SubTaskState),
			CompletedTasks:   make([]string, 0),
			FailedTasks:      make([]string, 0),
			AggregatedOutput: make(map[string]*core.Output),
		}
	}
	s.ParallelState.SubTasks[subTask.TaskID] = subTask
	return nil
}

// UpdateSubTaskStatus updates the status of a sub-task
func (s *State) UpdateSubTaskStatus(taskID string, status core.StatusType, output *core.Output, err *core.Error) error {
	if !s.IsParallel() || s.ParallelState == nil {
		return fmt.Errorf("cannot update sub-task in non-parallel execution")
	}

	subTask, exists := s.ParallelState.SubTasks[taskID]
	if !exists {
		return fmt.Errorf("sub-task %s not found", taskID)
	}

	subTask.Status = status
	subTask.Output = output
	subTask.Error = err
	now := time.Now()
	subTask.CompletedAt = &now

	// Update tracking lists
	switch status {
	case core.StatusSuccess:
		s.ParallelState.CompletedTasks = append(s.ParallelState.CompletedTasks, taskID)
		if output != nil {
			s.ParallelState.AggregatedOutput[taskID] = output
		}
	case core.StatusFailed:
		s.ParallelState.FailedTasks = append(s.ParallelState.FailedTasks, taskID)
	}

	// Update overall status based on strategy
	s.updateOverallStatus()
	return nil
}

// updateOverallStatus determines the overall parallel task status based on strategy
func (s *State) updateOverallStatus() {
	if !s.IsParallel() || s.ParallelState == nil {
		return
	}

	totalTasks := len(s.ParallelState.SubTasks)
	completedCount := len(s.ParallelState.CompletedTasks)
	failedCount := len(s.ParallelState.FailedTasks)

	switch s.ParallelState.Strategy {
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
	if completedCount == totalTasks {
		s.Status = core.StatusSuccess
	} else if failedCount > 0 && (completedCount+failedCount) == totalTasks {
		s.Status = core.StatusFailed
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

// GetAggregatedOutput returns the combined output from all completed sub-tasks
func (s *State) GetAggregatedOutput() map[string]*core.Output {
	if !s.IsParallel() || s.ParallelState == nil {
		return nil
	}
	return s.ParallelState.AggregatedOutput
}

// GetParallelProgress returns progress information for parallel execution
func (s *State) GetParallelProgress() (completed, failed, total int) {
	if !s.IsParallel() || s.ParallelState == nil {
		return 0, 0, 0
	}
	return len(s.ParallelState.CompletedTasks),
		len(s.ParallelState.FailedTasks),
		len(s.ParallelState.SubTasks)
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
	Component     core.ComponentType      `json:"component"`
	ExecutionType ExecutionType           `json:"execution_type"`
	AgentID       *string                 `json:"agent_id,omitempty"`
	ActionID      *string                 `json:"action_id,omitempty"`
	ToolID        *string                 `json:"tool_id,omitempty"`
	Input         *core.Input             `json:"input,omitempty"`
	MergedEnv     core.EnvMap             `json:"merged_env"`
	ParallelState *ParallelExecutionState `json:"parallel_state,omitempty"`
}

// -----------------------------------------------------------------------------
// Factory functions for creating states
// -----------------------------------------------------------------------------

// CreateBasicState creates a basic execution state
func CreateBasicState(input *StateInput, result *PartialState) *State {
	return &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      result.Component,
		Status:         core.StatusPending,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		ExecutionType:  ExecutionBasic,
		AgentID:        result.AgentID,
		ActionID:       result.ActionID,
		ToolID:         result.ToolID,
		Input:          result.Input,
		Output:         nil,
		Error:          nil,
	}
}

// CreateParallelState creates a parallel execution state
func CreateParallelState(input *StateInput, result *PartialState) *State {
	return &State{
		TaskID:         input.TaskID,
		TaskExecID:     input.TaskExecID,
		Component:      core.ComponentTask,
		Status:         core.StatusRunning,
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
		ExecutionType:  ExecutionParallel,
		ParallelState:  result.ParallelState,
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

// IsParallelExecution returns true if this response is for a parallel task
func (r *Response) IsParallelExecution() bool {
	return r.State != nil && r.State.IsParallel()
}

// GetParallelProgress returns progress information if this is a parallel task
func (r *Response) GetParallelProgress() (completed, failed, total int) {
	if r.State != nil && r.State.IsParallel() {
		return r.State.GetParallelProgress()
	}
	return 0, 0, 0
}

// ShouldContinueParallel returns true if the parallel execution should continue
func (r *Response) ShouldContinueParallel() bool {
	if !r.IsParallelExecution() {
		return false
	}

	// Continue if the task is still running or pending
	return r.State.Status == core.StatusRunning || r.State.Status == core.StatusPending
}
