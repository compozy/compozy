package agent

import (
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/state"
)

type State struct {
	state.BaseState
	TaskExecID     string `json:"task_exec_id"`
	WorkflowExecID string `json:"workflow_exec_id"`
}

func (as *State) UpdateFromEvent(ev nats.Event) error {
	st, err := nats.StatusFromEvent(ev)
	if err != nil {
		return err
	}

	as.SetStatus(st)

	switch st {
	case nats.StatusRunning:
		// TODO: Update the input and output of the agent
	case nats.StatusSuccess:
		// TODO: Update the input and output of the agent
	case nats.StatusFailed:
		// TODO: Update the input and output of the agent
	}

	return nil
}

func NewAgentState(agentID, execID, taskExecID, workflowExecID string) *State {
	return &State{
		BaseState: state.BaseState{
			Status: "PENDING",
			ID:     state.NewID(nats.ComponentAgent, agentID, execID),
			Input:  make(map[string]any),
			Output: make(map[string]any),
			Env:    make(map[string]string),
		},
		TaskExecID:     taskExecID,
		WorkflowExecID: workflowExecID,
	}
}
