package tool

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
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
	env, err := ti.TaskEnv.Merge(ti.ToolEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ti.ToolInput.Merge(*ti.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bs := &state.BaseState{
		StateID: GetToolStateID(ti.Context.Metadata),
		Status:  nats.StatusPending,
		Trigger: ti.TriggerInput,
		Input:   &input,
		Output:  &common.Output{},
		Env:     &env,
		Error:   nil,
	}
	state := &State{
		BaseState: *bs,
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
		return nil, fmt.Errorf("failed to initialize tool state: %w", err)
	}
	return state, nil
}

func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.EventToolStarted:
		return s.handleStartedEvent(evt)
	case *pb.EventToolSuccess:
		return s.handleSuccessEvent(evt)
	case *pb.EventToolFailed:
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for tool state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.EventToolStarted) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.EventToolSuccess) error {
	s.Status = nats.StatusSuccess
	state.SetStateResult(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.EventToolFailed) error {
	s.Status = nats.StatusFailed
	state.SetStateError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
