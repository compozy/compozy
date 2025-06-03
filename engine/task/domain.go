package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type StateID struct {
	TaskID     string  `json:"task_id" db:"task_id"`
	TaskExecID core.ID `json:"task_exec_id" db:"task_exec_id"`
}

func (e *StateID) GetComponentID() string {
	return e.TaskID
}

func (e *StateID) GetExecID() core.ID {
	return e.TaskExecID
}

func (e *StateID) String() string {
	return fmt.Sprintf("%s_%s", e.TaskID, e.TaskExecID)
}

func (e *StateID) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func UnmarshalStateID(data []byte) (*StateID, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	parts := strings.Split(s, "_")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid state ID format: %s", s)
	}
	return &StateID{TaskID: parts[0], TaskExecID: core.ID(parts[1])}, nil
}

type State struct {
	StateID
	Component      core.ComponentType `json:"component" db:"component"`
	Status         core.StatusType    `json:"status" db:"status"`
	WorkflowID     string             `json:"workflow_id" db:"workflow_id"`
	WorkflowExecID core.ID            `json:"workflow_exec_id" db:"workflow_exec_id"`
	AgentID        *string            `json:"agent_id,omitempty" db:"agent_id"`
	ToolID         *string            `json:"tool_id,omitempty" db:"tool_id"`
	Input          *core.Input        `json:"input,omitempty" db:"input"`
	Output         *core.Output       `json:"output,omitempty" db:"output"`
	Error          *core.Error        `json:"error,omitempty" db:"error"`
}

// StateDB is used for database scanning with JSONB fields as []byte
type StateDB struct {
	StateID
	Component      core.ComponentType `db:"component"`
	Status         core.StatusType    `db:"status"`
	WorkflowID     string             `db:"workflow_id"`
	WorkflowExecID core.ID            `db:"workflow_exec_id"`
	AgentIDRaw     sql.NullString     `db:"agent_id"` // Can be NULL
	ToolIDRaw      sql.NullString     `db:"tool_id"`  // Can be NULL
	InputRaw       []byte             `db:"input"`
	OutputRaw      []byte             `db:"output"`
	ErrorRaw       []byte             `db:"error"`
}

// ToState converts StateDB to State with proper JSON unmarshaling
func (sdb *StateDB) ToState() (*State, error) {
	state := &State{
		StateID:        sdb.StateID,
		WorkflowID:     sdb.WorkflowID,
		WorkflowExecID: sdb.WorkflowExecID,
		Status:         sdb.Status,
		Component:      core.ComponentTask,
	}

	// Handle nullable AgentID
	if sdb.AgentIDRaw.Valid {
		state.AgentID = &sdb.AgentIDRaw.String
		state.Component = core.ComponentAgent
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
