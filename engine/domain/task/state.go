package task

import (
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/state"
)

type State struct {
	state.BaseState
	WorkflowExecID string `json:"workflow_exec_id"`
}

func (ts *State) UpdateFromEvent(event nats.Event) error {
	st, err := nats.StatusFromEvent(event)
	if err != nil {
		return err
	}

	ts.SetStatus(st)

	switch st {
	case nats.StatusRunning:
		// TODO: Update the input and output of the task
	case nats.StatusSuccess:
		// TODO: Update the input and output of the task
	case nats.StatusFailed:
		// TODO: Update the input and output of the task
	case nats.StatusWaiting:
		// TODO: Update the input and output of the task
	case nats.StatusRetryScheduled:
		// TODO: Update the input and output of the task
	case nats.StatusCancelled:
		// TODO: Update the input and output of the task
	case nats.StatusTimedOut:
		// TODO: Update the input and output of the task
	case nats.StatusScheduled:
		// TODO: Update the input and output of the task
	case nats.StatusPending:
		// TODO: Update the input and output of the task
	}

	return nil
}

func NewTaskState(taskID, execID, workflowExecID string) *State {
	return &State{
		BaseState: state.BaseState{
			Status: nats.StatusPending,
			ID:     state.NewID(nats.ComponentTask, taskID, execID),
			Input:  make(map[string]any),
			Output: make(map[string]any),
			Env:    make(map[string]string),
		},
		WorkflowExecID: workflowExecID,
	}
}
