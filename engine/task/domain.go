package task

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type StateID struct {
	TaskID     string
	TaskExecID core.ID
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
	return &StateID{TaskID: parts[0], TaskExecID: core.ID(parts[1])}, nil
}

type State struct {
	Status    core.StatusType    `json:"status"`
	Component core.ComponentType `json:"component"`
	StateID   StateID            `json:"state_id"`
	AgentID   *string            `json:"agent_id,omitempty"`
	ToolID    *string            `json:"tool_id,omitempty"`
	Input     *core.Input        `json:"input,omitempty"`
	Output    *core.Output       `json:"output,omitempty"`
	Error     *core.Error        `json:"error,omitempty"`
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
