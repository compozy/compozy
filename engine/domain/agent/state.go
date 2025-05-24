package agent

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

func (ai *StateInitializer) Initialize() (*State, error) {
	env, err := ai.TaskEnv.Merge(ai.AgentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ai.AgentInput.Merge(*ai.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bsState := &state.BaseState{
		StateID: GetAgentStateID(ai.Metadata),
		Status:  nats.StatusPending,
		Trigger: ai.TriggerInput,
		Input:   &input,
		Output:  &common.Output{},
		Env:     &env,
		Error:   nil,
	}
	state := &State{
		BaseState: *bsState,
		Context:   ai.Context,
	}
	if err := ai.Normalizer.ParseTemplates(state); err != nil {
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
		return nil, fmt.Errorf("failed to initialize agent state: %w", err)
	}
	return state, nil
}

func (s *State) GetContext() *Context {
	return s.Context
}

func (s *State) GetComponentID() string {
	return s.Context.GetAgentID()
}

func (s *State) UpdateFromEvent(event any) error {
	metadata := s.Context.GetMetadata()
	switch evt := event.(type) {
	case *pb.EventAgentStarted:
		logger.Debug("Received: AgentStarted", "metadata", metadata)
		return s.handleStartedEvent(evt)
	case *pb.EventAgentSuccess:
		logger.Debug("Received: AgentSuccess", "metadata", metadata)
		return s.handleSuccessEvent(evt)
	case *pb.EventAgentFailed:
		logger.Debug("Received: AgentFailed", "metadata", metadata)
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for agent state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.EventAgentStarted) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.EventAgentSuccess) error {
	s.Status = nats.StatusSuccess
	state.SetStateResult(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.EventAgentFailed) error {
	s.Status = nats.StatusFailed
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
