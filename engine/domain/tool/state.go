package tool

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	pb "github.com/compozy/compozy/pkg/pb/tool"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type StateParams struct {
	CorrID         common.CorrID
	WorkflowExecID common.ExecID
	TaskExecID     common.ExecID
	ExecID         common.ExecID
	TaskEnv        common.EnvMap
	ToolEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	ToolInput      *common.Input
}

func NewStateParams(
	corrID common.CorrID,
	taskExecID, workflowExecID common.ExecID,
	taskEnv, toolEnv common.EnvMap,
	tgInput, taskInput, toolInput *common.Input,
) *StateParams {
	execID := common.NewExecID()
	return &StateParams{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
		ExecID:         execID,
		TaskEnv:        taskEnv,
		ToolEnv:        toolEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		ToolInput:      toolInput,
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
	env, err := ti.MergeEnv(ti.TaskEnv, ti.ToolEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ti.ToolInput.Merge(*ti.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bs := &state.BaseState{
		StateID: state.NewID(nats.ComponentTool, ti.CorrID, ti.ExecID),
		Status:  nats.StatusPending,
		Input:   &input,
		Output:  &common.Output{},
		Env:     env,
		Trigger: ti.TriggerInput,
	}
	st := &State{
		BaseState:      *bs,
		WorkflowExecID: ti.WorkflowExecID,
		TaskExecID:     ti.TaskExecID,
		ToolExecID:     ti.ExecID,
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
	ToolExecID     common.ExecID `json:"tool_exec_id"`
}

func NewToolState(exec *StateParams) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		StateParams:       exec,
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
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}

func (s *State) handleFailedEvent(evt *pb.ToolExecutionFailedEvent) error {
	s.Status = nats.StatusFailed
	if evt.GetPayload() == nil || evt.GetPayload().GetResult() == nil {
		return nil
	}
	state.SetResultData(&s.BaseState, evt.GetPayload().GetResult())
	return nil
}
