package task

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
)

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	CorrID         common.CorrID
	WorkflowExecID common.ExecID
	TaskExecID     common.ExecID
	WorkflowEnv    common.EnvMap
	TaskEnv        common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
}

func NewExecution(
	corrID common.CorrID,
	workflowExecID common.ExecID,
	workflowEnv, taskEnv common.EnvMap,
	tgInput, taskInput *common.Input,
) *Execution {
	execID := common.NewExecID()
	return &Execution{
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

type TaskStateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (ti *TaskStateInitializer) Initialize() (*State, error) {
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

func NewTaskState(exec *Execution) (*State, error) {
	initializer := &TaskStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task state: %w", err)
	}
	return st, nil
}
