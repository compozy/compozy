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

type Context struct {
	CorrID         common.ID     `json:"correlation_id"`
	WorkflowExecID common.ID     `json:"workflow_execution_id"`
	TaskExecID     common.ID     `json:"task_execution_id"`
	ToolExecID     common.ID     `json:"tool_execution_id"`
	TaskEnv        common.EnvMap `json:"task_env"`
	ToolEnv        common.EnvMap `json:"tool_env"`
	TriggerInput   *common.Input `json:"trigger_input"`
	TaskInput      *common.Input `json:"task_input"`
	ToolInput      *common.Input `json:"tool_input"`
}

func NewContext(
	corrID common.ID,
	taskExecID, workflowExecID common.ID,
	taskEnv, toolEnv common.EnvMap,
	tgInput, taskInput, toolInput *common.Input,
) (*Context, error) {
	execID, err := common.NewID()
	if err != nil {
		return nil, err
	}
	return &Context{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
		ToolExecID:     execID,
		TaskEnv:        taskEnv,
		ToolEnv:        toolEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		ToolInput:      toolInput,
	}, nil
}

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type StateInitializer struct {
	*state.CommonInitializer
	*Context
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
		Context:   ti.Context,
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
	Context *Context `json:"context,omitempty"`
}

func NewToolState(stCtx *Context) (*State, error) {
	initializer := &StateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Context:           stCtx,
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
