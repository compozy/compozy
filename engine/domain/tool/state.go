package tool

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/tool"
)

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (ti *StateInitializer) Initialize() (*State, error) {
	env, err := ti.MergeEnv(ti.TaskEnv, ti.ToolEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ti.ToolInput.Merge(*ti.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bs := &state.BaseState{
		StateID: state.NewID(nats.ComponentTool, ti.CorrID, ti.ToolExecID),
		Status:  nats.StatusPending,
		Input:   &input,
		Output:  &common.Output{},
		Env:     env,
		Trigger: ti.TriggerInput,
	}
	st := &State{
		BaseState: *bs,
		Execution: ti.Execution,
	}
	if err := ti.Normalizer.ParseTemplates(st); err != nil {
		return nil, err
	}
	return st, nil
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	state.BaseState
	Execution *Execution `json:"execution,omitempty"`
}

func NewToolState(exec *Execution) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tool state: %w", err)
	}
	return st, nil
}

func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.ToolExecutionStartedEvent:
		return s.handleStartedEvent(evt)
	case *pb.ToolExecutionSuccessEvent:
		return s.handleSuccessEvent(evt)
	case *pb.ToolExecutionFailedEvent:
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for tool state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.ToolExecutionStartedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.ToolExecutionSuccessEvent) error {
	s.Status = nats.StatusSuccess
	state.SetResultData(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.ToolExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	state.SetResultError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
