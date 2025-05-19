package workflow

import (
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/state"
)

// State represents the state of a workflow execution
type State struct {
	state.BaseState
	Tasks state.Map `json:"tasks,omitempty"`
}

// UpdateFromEvent updates the workflow state based on an event
func (ws *State) UpdateFromEvent(event nats.Event) error {
	st, err := nats.StatusFromEvent(event)
	if err != nil {
		return err
	}

	ws.SetStatus(st)

	switch st {
	case nats.StatusRunning:
		// TODO: Update the input and output of the workflow
	case nats.StatusSuccess:
		// TODO: Update the input and output of the workflow
	case nats.StatusFailed:
		// TODO: Update the input and output of the workflow
	case nats.StatusWaiting:
		// TODO: Update the input and output of the workflow
	case nats.StatusCancelled:
		// TODO: Update the input and output of the workflow
	case nats.StatusTimedOut:
		// TODO: Update the input and output of the workflow
	case nats.StatusPending:
		// TODO: Update the input and output of the workflow
	case nats.StatusScheduled:
		// TODO: Update the input and output of the workflow
	}

	return nil
}

// NewWorkflowState creates a new workflow state
func NewWorkflowState(wID, cID string) *State {
	return &State{
		BaseState: state.BaseState{
			Status: nats.StatusPending,
			ID:     state.NewID(nats.ComponentWorkflow, wID, cID),
			Input:  make(map[string]any),
			Output: make(map[string]any),
			Env:    make(map[string]string),
		},
		Tasks: make(state.Map),
	}
}
