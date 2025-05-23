package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/workflow"
)

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Context
}

func (wi *StateInitializer) Initialize() (*State, error) {
	env, err := wi.ProjectEnv.Merge(wi.WorkflowEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentWorkflow, wi.CorrID, wi.WorkflowExecID),
		Status:  nats.StatusPending,
		Input:   wi.TriggerInput,
		Output:  &common.Output{},
		Trigger: wi.TriggerInput,
		Env:     &env,
		Error:   nil,
	}
	state := &State{
		BaseState: *bsState,
		Context:   wi.Context,
	}
	if err := wi.Normalizer.ParseTemplates(state); err != nil {
		return nil, err
	}
	return state, nil
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	state.BaseState
	Context *Context `json:"context,omitempty"`
}

func NewState(ctx *Context) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Context:           ctx,
	}
	state, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize workflow state: %w", err)
	}
	return state, nil
}

func (s *State) GetContext() *Context {
	return s.Context
}

func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.EventWorkflowStarted:
		return s.handleStartedEvent(evt)
	case *pb.EventWorkflowPaused:
		return s.handlePausedEvent(evt)
	case *pb.EventWorkflowResumed:
		return s.handleResumedEvent(evt)
	case *pb.EventWorkflowSuccess:
		return s.handleSuccessEvent(evt)
	case *pb.EventWorkflowFailed:
		return s.handleFailedEvent(evt)
	case *pb.EventWorkflowCanceled:
		return s.handleCanceledEvent(evt)
	case *pb.EventWorkflowTimedOut:
		return s.handleTimedOutEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for workflow state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.EventWorkflowStarted) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handlePausedEvent(_ *pb.EventWorkflowPaused) error {
	s.Status = nats.StatusPaused
	return nil
}

func (s *State) handleResumedEvent(_ *pb.EventWorkflowResumed) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.EventWorkflowSuccess) error {
	s.Status = nats.StatusSuccess
	state.SetStateResult(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.EventWorkflowFailed) error {
	s.Status = nats.StatusFailed
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}

func (s *State) handleCanceledEvent(_ *pb.EventWorkflowCanceled) error {
	s.Status = nats.StatusCanceled
	return nil
}

func (s *State) handleTimedOutEvent(evt *pb.EventWorkflowTimedOut) error {
	s.Status = nats.StatusTimedOut
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
