package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
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
		StateID: GetWorkflowStateID(wi.Metadata),
		Status:  nats.StatusPending,
		Trigger: wi.TriggerInput,
		Input:   wi.TriggerInput,
		Output:  &common.Output{},
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

func (s *State) GetComponentID() string {
	return s.Context.GetWorkflowID()
}

func (s *State) UpdateFromEvent(event any) error {
	metadata := s.Context.GetMetadata()
	switch evt := event.(type) {
	case *pb.EventWorkflowStarted:
		logger.Debug("Received: WorkflowStarted", "metadata", metadata)
		return s.handleStartedEvent(evt)
	case *pb.EventWorkflowPaused:
		logger.Debug("Received: WorkflowPaused", "metadata", metadata)
		return s.handlePausedEvent(evt)
	case *pb.EventWorkflowResumed:
		logger.Debug("Received: WorkflowResumed", "metadata", metadata)
		return s.handleResumedEvent(evt)
	case *pb.EventWorkflowSuccess:
		logger.Debug("Received: WorkflowSuccess", "metadata", metadata)
		return s.handleSuccessEvent(evt)
	case *pb.EventWorkflowFailed:
		logger.Debug("Received: WorkflowFailed", "metadata", metadata)
		return s.handleFailedEvent(evt)
	case *pb.EventWorkflowCanceled:
		logger.Debug("Received: WorkflowCanceled", "metadata", metadata)
		return s.handleCanceledEvent(evt)
	case *pb.EventWorkflowTimedOut:
		logger.Debug("Received: WorkflowTimedOut", "metadata", metadata)
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
