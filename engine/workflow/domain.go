package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

const StateKey = "workflowState"

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	WorkflowID     string                 `json:"workflow_id"      db:"workflow_id"`
	WorkflowExecID core.ID                `json:"workflow_exec_id" db:"workflow_exec_id"`
	Status         core.StatusType        `json:"status"           db:"status"`
	Input          *core.Input            `json:"input"            db:"input"`
	Output         *core.Output           `json:"output"           db:"output"`
	Error          *core.Error            `json:"error"            db:"error"`
	Tasks          map[string]*task.State `json:"tasks"`
}

// StateDB is used for database scanning with JSONB fields as []byte
type StateDB struct {
	WorkflowID     string          `db:"workflow_id"`
	WorkflowExecID core.ID         `db:"workflow_exec_id"`
	Status         core.StatusType `db:"status"`
	InputRaw       []byte          `db:"input"`
	OutputRaw      []byte          `db:"output"`
	ErrorRaw       []byte          `db:"error"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// ToState converts StateDB to State with proper JSON unmarshaling
func (sdb *StateDB) ToState() (*State, error) {
	state := &State{
		WorkflowID:     sdb.WorkflowID,
		WorkflowExecID: sdb.WorkflowExecID,
		Status:         sdb.Status,
		Tasks:          make(map[string]*task.State),
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

func NewState(workflowID string, workflowExecID core.ID, input *core.Input) *State {
	return &State{
		Status:         core.StatusRunning,
		WorkflowID:     workflowID,
		WorkflowExecID: workflowExecID,
		Input:          input,
		Tasks:          make(map[string]*task.State),
		Output:         nil,
		Error:          nil,
	}
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

func (e *State) WithStatus(status core.StatusType) *State {
	e.Status = status
	return e
}

func (e *State) WithError(err *core.Error) *State {
	e.Error = err
	return e
}

func (e *State) WithOutput(output *core.Output) *State {
	e.Output = output
	return e
}

func (e *State) AddTask(task *task.State) {
	e.Tasks[task.TaskExecID.String()] = task
}

func (e *State) GetTask(taskExecID core.ID) *task.State {
	return e.Tasks[taskExecID.String()]
}

func (e *State) GetTaskByID(taskID string) *task.State {
	for _, task := range e.Tasks {
		if task.TaskID == taskID {
			return task
		}
	}
	return nil
}

func (e *State) GetTaskByExecID(taskExecID core.ID) *task.State {
	for _, task := range e.Tasks {
		if task.TaskExecID == taskExecID {
			return task
		}
	}
	return nil
}

func (e *State) GetByAgentID(agentID string) *task.State {
	for _, task := range e.Tasks {
		if task.AgentID != nil && *task.AgentID == agentID {
			return task
		}
	}
	return nil
}

func (e *State) GetByToolID(toolID string) *task.State {
	for _, task := range e.Tasks {
		if task.ToolID != nil && *task.ToolID == toolID {
			return task
		}
	}
	return nil
}
