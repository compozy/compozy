package tool

import (
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/state"
)

// State represents the state of a tool execution
type State struct {
	state.BaseState
	TaskExecID     string `json:"task_exec_id"`
	WorkflowExecID string `json:"workflow_exec_id"`
}

// UpdateFromEvent updates the tool state based on an event
func (ts *State) UpdateFromEvent(event nats.Event) error {
	st, err := nats.StatusFromEvent(event)
	if err != nil {
		return err
	}

	ts.SetStatus(st)

	switch st {
	case nats.StatusRunning:
		// TODO: Update the input and output of the tool
	case nats.StatusSuccess:
		// TODO: Update the input and output of the tool
	case nats.StatusFailed:
		// TODO: Update the input and output of the tool
	}

	return nil
}

func NewToolState(toolID, execID, taskExecID, workflowExecID string) *State {
	return &State{
		BaseState: state.BaseState{
			Status: nats.StatusPending,
			ID:     state.NewID(nats.ComponentTool, toolID, execID),
			Input:  make(map[string]any),
			Output: make(map[string]any),
			Env:    make(map[string]string),
		},
		TaskExecID:     taskExecID,
		WorkflowExecID: workflowExecID,
	}
}
