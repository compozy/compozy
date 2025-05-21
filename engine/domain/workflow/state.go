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
	*Execution
}

func (wi *StateInitializer) Initialize() (*State, error) {
	env, err := wi.MergeEnv(wi.ProjectEnv, wi.WorkflowEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentWorkflow, wi.CorrID, wi.WorkflowExecID),
		Status:  nats.StatusPending,
		Input:   &common.Input{},
		Output:  &common.Output{},
		Trigger: wi.TriggerInput,
		Env:     env,
		Error:   nil,
	}
	st := &State{
		BaseState:      *bsState,
		WorkflowExecID: wi.WorkflowExecID,
	}
	if err := wi.Normalizer.ParseTemplates(st); err != nil {
		return nil, err
	}
	return st, nil
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

type State struct {
	state.BaseState
	WorkflowExecID common.ID `json:"workflow_exec_id,omitempty"`
}

func NewState(exec *Execution) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize workflow state: %w", err)
	}
	return st, nil
}

func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.WorkflowExecutionStartedEvent:
		return s.handleStartedEvent(evt)
	case *pb.WorkflowExecutionPausedEvent:
		return s.handlePausedEvent(evt)
	case *pb.WorkflowExecutionResumedEvent:
		return s.handleResumedEvent(evt)
	case *pb.WorkflowExecutionSuccessEvent:
		return s.handleSuccessEvent(evt)
	case *pb.WorkflowExecutionFailedEvent:
		return s.handleFailedEvent(evt)
	case *pb.WorkflowExecutionCanceledEvent:
		return s.handleCanceledEvent(evt)
	case *pb.WorkflowExecutionTimedOutEvent:
		return s.handleTimedOutEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for workflow state update: %T", evt)
	}
}

func (s *State) handleStartedEvent(_ *pb.WorkflowExecutionStartedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handlePausedEvent(_ *pb.WorkflowExecutionPausedEvent) error {
	s.Status = nats.StatusPaused
	return nil
}

func (s *State) handleResumedEvent(_ *pb.WorkflowExecutionResumedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.WorkflowExecutionSuccessEvent) error {
	s.Status = nats.StatusSuccess
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.WorkflowExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleCanceledEvent(_ *pb.WorkflowExecutionCanceledEvent) error {
	s.Status = nats.StatusCanceled
	return nil
}

func (s *State) handleTimedOutEvent(evt *pb.WorkflowExecutionTimedOutEvent) error {
	s.Status = nats.StatusTimedOut
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}
