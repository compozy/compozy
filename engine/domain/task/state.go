package task

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/task"
)

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (ti *StateInitializer) Initialize() (*State, error) {
	env, err := ti.MergeEnv(ti.WorkflowEnv, ti.TaskEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentTask, ti.CorrID, ti.TaskExecID),
		Status:  nats.StatusPending,
		Input:   ti.TaskInput,
		Output:  &common.Output{},
		Trigger: ti.TriggerInput,
		Env:     env,
	}
	st := &State{
		BaseState:      *bsState,
		WorkflowExecID: ti.WorkflowExecID,
		TaskExecID:     ti.TaskExecID,
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
	WorkflowExecID common.ID `json:"workflow_exec_id"`
	TaskExecID     common.ID `json:"task_exec_id"`
}

func NewTaskState(exec *Execution) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task state: %w", err)
	}
	return st, nil
}

func (s *State) UpdateFromEvent(event any) error {
	switch evt := event.(type) {
	case *pb.TaskDispatchedEvent:
		return s.handleDispatchedEvent(evt)
	case *pb.TaskExecutionStartedEvent:
		return s.handleStartedEvent(evt)
	case *pb.TaskExecutionWaitingStartedEvent:
		return s.handleWaitingStartedEvent(evt)
	case *pb.TaskExecutionWaitingEndedEvent:
		return s.handleWaitingEndedEvent(evt)
	case *pb.TaskExecutionWaitingTimedOutEvent:
		return s.handleWaitingTimedOutEvent(evt)
	case *pb.TaskExecutionSuccessEvent:
		return s.handleSuccessEvent(evt)
	case *pb.TaskExecutionFailedEvent:
		return s.handleFailedEvent(evt)
	default:
		return fmt.Errorf("unsupported event type for task state update: %T", evt)
	}
}

func (s *State) handleDispatchedEvent(_ *pb.TaskDispatchedEvent) error {
	s.Status = nats.StatusPending
	return nil
}

func (s *State) handleStartedEvent(_ *pb.TaskExecutionStartedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleWaitingStartedEvent(_ *pb.TaskExecutionWaitingStartedEvent) error {
	s.Status = nats.StatusWaiting
	return nil
}

func (s *State) handleWaitingEndedEvent(_ *pb.TaskExecutionWaitingEndedEvent) error {
	s.Status = nats.StatusRunning
	return nil
}

func (s *State) handleWaitingTimedOutEvent(evt *pb.TaskExecutionWaitingTimedOutEvent) error {
	s.Status = nats.StatusTimedOut
	state.SetResultError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.TaskExecutionSuccessEvent) error {
	s.Status = nats.StatusSuccess
	state.SetResultData(&s.BaseState, evt.GetDetails().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.TaskExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	state.SetResultError(&s.BaseState, evt.GetDetails().GetError())
	return nil
}
