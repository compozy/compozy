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
// State
// -----------------------------------------------------------------------------

type State struct {
	Component      core.ComponentType `json:"component"           db:"component"`
	Status         core.StatusType    `json:"status"              db:"status"`
	TaskID         string             `json:"task_id"             db:"task_id"`
	TaskExecID     core.ID            `json:"task_exec_id"        db:"task_exec_id"`
	WorkflowID     string             `json:"workflow_id"         db:"workflow_id"`
	WorkflowExecID core.ID            `json:"workflow_exec_id"    db:"workflow_exec_id"`
	AgentID        *string            `json:"agent_id,omitempty"  db:"agent_id"`
	ActionID       *string            `json:"action_id,omitempty" db:"action_id"`
	ToolID         *string            `json:"tool_id,omitempty"   db:"tool_id"`
	Input          *core.Input        `json:"input,omitempty"     db:"input"`
	Output         *core.Output       `json:"output,omitempty"    db:"output"`
	Error          *core.Error        `json:"error,omitempty"     db:"error"`
}

// StateDB is used for database scanning with JSONB fields as []byte
type StateDB struct {
	Component      core.ComponentType `db:"component"`
	Status         core.StatusType    `db:"status"`
	TaskID         string             `db:"task_id"`
	TaskExecID     core.ID            `db:"task_exec_id"`
	WorkflowID     string             `db:"workflow_id"`
	WorkflowExecID core.ID            `db:"workflow_exec_id"`
	AgentIDRaw     sql.NullString     `db:"agent_id"`  // Can be NULL
	ActionIDRaw    sql.NullString     `db:"action_id"` // Can be NULL
	ToolIDRaw      sql.NullString     `db:"tool_id"`   // Can be NULL
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
		Component:      core.ComponentTask,
	}

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

	// Unmarshal input
	if sdb.InputRaw != nil {
		var input core.Input
		if err := json.Unmarshal(sdb.InputRaw, &input); err != nil {
			return nil, fmt.Errorf("unmarshaling input: %w", err)
		}
		state.Input = &input
	}

	// Unmarshal output
	if sdb.OutputRaw != nil {
		var output core.Output
		if err := json.Unmarshal(sdb.OutputRaw, &output); err != nil {
			return nil, fmt.Errorf("unmarshaling output: %w", err)
		}
		state.Output = &output
	}

	// Unmarshal error
	if sdb.ErrorRaw != nil {
		var errorObj core.Error
		if err := json.Unmarshal(sdb.ErrorRaw, &errorObj); err != nil {
			return nil, fmt.Errorf("unmarshaling error: %w", err)
		}
		state.Error = &errorObj
	}

	return state, nil
}

func (e *State) AsMap() (map[core.ID]any, error) {
	val, err := json.Marshal(e)
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

func (e *State) UpdateStatus(status core.StatusType) {
	e.Status = status
}

// -----------------------------------------------------------------------------
// Response
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
