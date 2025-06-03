package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type StateID struct {
	WorkflowID   string
	WorkflowExec core.ID
}

func StateIDFromString(s string) (*StateID, error) {
	parts := strings.Split(s, "_")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid state ID: %s", s)
	}
	return &StateID{WorkflowID: parts[0], WorkflowExec: core.ID(parts[1])}, nil
}

func (e *StateID) GetComponentID() string {
	return e.WorkflowID
}

func (e *StateID) GetExecID() core.ID {
	return e.WorkflowExec
}

func (e *StateID) String() string {
	return fmt.Sprintf("%s_%s", e.WorkflowID, e.WorkflowExec)
}

func (e *StateID) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e *StateID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	e.WorkflowID, e.WorkflowExec = strings.Split(s, "_")[0], core.ID(strings.Split(s, "_")[1])
	return nil
}

type State struct {
	Status  core.StatusType        `json:"status"`
	StateID StateID                `json:"state_id"`
	Input   *core.Input            `json:"input"`
	Output  *core.Output           `json:"output"`
	Error   *core.Error            `json:"error"`
	Tasks   map[string]*task.State `json:"tasks"`
}

func NewState(workflowID string, workflowExecID core.ID, input *core.Input) *State {
	stateID := StateID{
		WorkflowID:   workflowID,
		WorkflowExec: workflowExecID,
	}
	return &State{
		Status:  core.StatusRunning,
		StateID: stateID,
		Input:   input,
		Tasks:   make(map[string]*task.State),
		Output:  nil,
		Error:   nil,
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

func (e *State) UpdateStatus(status core.StatusType) {
	e.Status = status
}

func (e *State) AddTask(task *task.State) {
	e.Tasks[task.StateID.String()] = task
}

func (e *State) GetTask(taskID task.StateID) *task.State {
	return e.Tasks[taskID.String()]
}

func (e *State) GetTaskByID(taskID string) *task.State {
	for _, task := range e.Tasks {
		if task.StateID.TaskID == taskID {
			return task
		}
	}
	return nil
}

func (e *State) GetTaskByExecID(taskExecID core.ID) *task.State {
	for _, task := range e.Tasks {
		if task.StateID.TaskExecID == taskExecID {
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
