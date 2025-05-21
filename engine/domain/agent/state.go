package agent

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
	AgentExecID    common.ExecID
	TaskEnv        common.EnvMap
	AgentEnv       common.EnvMap
	TriggerInput   *common.Input
	TaskInput      *common.Input
	AgentInput     *common.Input
}

func NewExecution(
	corrID common.CorrID,
	taskExecID, workflowExecID common.ExecID,
	taskEnv, agentEnv common.EnvMap,
	tgInput, taskInput, agentInput *common.Input,
) *Execution {
	execID := common.NewExecID()
	return &Execution{
		CorrID:         corrID,
		WorkflowExecID: workflowExecID,
		TaskExecID:     taskExecID,
		AgentExecID:    execID,
		TaskEnv:        taskEnv,
		AgentEnv:       agentEnv,
		TriggerInput:   tgInput,
		TaskInput:      taskInput,
		AgentInput:     agentInput,
	}
}

// -----------------------------------------------------------------------------
// Initializer
// -----------------------------------------------------------------------------

type AgentStateInitializer struct {
	*state.CommonInitializer
	*Execution
}

func (ai *AgentStateInitializer) Initialize() (*State, error) {
	env, err := ai.MergeEnv(ai.TaskEnv, ai.AgentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	input, err := ai.AgentInput.Merge(*ai.TaskInput)
	if err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	bsState := &state.BaseState{
		StateID: state.NewID(nats.ComponentAgent, ai.CorrID, ai.AgentExecID),
		Status:  nats.StatusPending,
		Input:   &input,
		Output:  &common.Output{},
		Trigger: ai.TriggerInput,
		Env:     env,
	}
	st := &State{
		BaseState:      *bsState,
		WorkflowExecID: ai.WorkflowExecID,
		TaskExecID:     ai.TaskExecID,
		AgentExecID:    ai.AgentExecID,
	}
	if err := ai.Normalizer.ParseTemplates(st); err != nil {
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
	AgentExecID    common.ExecID `json:"agent_exec_id"`
}

func NewAgentState(exec *Execution) (*State, error) {
	initializer := &AgentStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		Execution:         exec,
	}
	st, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent state: %w", err)
	}
	return st, nil
}
