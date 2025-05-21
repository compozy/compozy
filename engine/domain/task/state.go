package task

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/task"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type StateParams struct {
	CorrID         common.CorrID
	WorkflowExecID common.ExecID
	TaskExecID     common.ExecID
	WorkflowEnv    common.EnvMap
	TaskEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
}

func NewStateParams(
	corrID common.CorrID,
	workflowExecID common.ExecID,
	workflowEnv, taskEnv common.EnvMap,
	tgInput, taskInput *common.Input,
) *StateParams {
	execID := common.NewExecID()
	return &StateParams{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     execID,
		WorkflowEnv:    workflowEnv,
		TaskEnv:        taskEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
	}
}

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*StateParams
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
	WorkflowExecID common.ExecID `json:"workflow_exec_id"`
	TaskExecID     common.ExecID `json:"task_exec_id"`
}

func NewTaskState(exec *StateParams) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		StateParams:       exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task state: %w", err)
	}
	return st, nil
}

// UpdateFromEvent updates the task state based on the event type
// It updates both the status and output data from the event
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
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleSuccessEvent(evt *pb.TaskExecutionSuccessEvent) error {
	s.Status = nats.StatusSuccess
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.TaskExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}
