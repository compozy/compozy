package task

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

func (ti *StateInitializer) Initialize() (*State, error) {
	env, err := ti.WorkflowEnv.Merge(ti.TaskEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	bsState := &state.BaseState{
		StateID: GetTaskStateID(ti.Metadata),
		Status:  nats.StatusPending,
		Trigger: ti.TriggerInput,
		Input:   ti.TaskInput,
		Output:  &common.Output{},
		Env:     &env,
		Error:   nil,
	}
	state := &State{
		BaseState: *bsState,
		Context:   ti.Context,
	}
	if err := ti.Normalizer.ParseTemplates(state); err != nil {
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

func NewState(stCtx *Context) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Context:           stCtx,
	}
	state, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task state: %w", err)
	}
	return state, nil
}

func (s *State) GetContext() *Context {
	return s.Context
}

func (s *State) GetComponentID() string {
	return s.Context.GetTaskID()
}

func (s *State) UpdateFromEvent(event any) error {
	metadata := s.Context.GetMetadata()
	switch evt := event.(type) {
	case *pb.EventTaskDispatched:
		logger.Debug("Received: TaskDispatched", "metadata", metadata)
		return s.handleDispatchedEvent(evt)
	case *pb.EventTaskStarted:
		logger.Debug("Received: TaskStarted", "metadata", metadata)
		return s.handleStartedEvent(evt)
	case *pb.EventTaskWaiting:
		logger.Debug("Received: TaskWaiting", "metadata", metadata)
		return s.handleWaitingStartedEvent(evt)
	case *pb.EventTaskWaitingEnded:
		logger.Debug("Received: TaskWaitingEnded", "metadata", metadata)
		return s.handleWaitingEndedEvent(evt)
	case *pb.EventTaskWaitingTimedOut:
		logger.Debug("Received: TaskWaitingTimedOut", "metadata", metadata)
		return s.handleWaitingTimedOutEvent(evt)
	case *pb.EventTaskSuccess:
		logger.Debug("Received: TaskSuccess", "metadata", metadata)
		return s.handleSuccessEvent(evt)
	case *pb.EventTaskFailed:
		logger.Debug("Received: TaskFailed", "metadata", metadata)
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for task state update: %T", evt)
	}
}

func (s *State) handleDispatchedEvent(_ *pb.EventTaskDispatched) error {
	s.Status = nats.StatusPending
	return nil
}

func (s *State) handleStartedEvent(_ *pb.EventTaskStarted) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleWaitingStartedEvent(_ *pb.EventTaskWaiting) error {
	s.Status = nats.StatusWaiting
	return nil
}

func (s *State) handleWaitingEndedEvent(_ *pb.EventTaskWaitingEnded) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleWaitingTimedOutEvent(evt *pb.EventTaskWaitingTimedOut) error {
	s.Status = nats.StatusTimedOut
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.EventTaskSuccess) error {
	s.Status = nats.StatusSuccess
	state.SetStateResult(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.EventTaskFailed) error {
	s.Status = nats.StatusFailed
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
