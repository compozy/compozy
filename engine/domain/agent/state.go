package agent

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
)

type State struct {
	state.BaseState
	TaskExecID     string `json:"task_exec_id"`
	WorkflowExecID string `json:"workflow_exec_id"`
}

func NewAgentState(
	execID string,
	tExecID string,
	wfExecID string,
	tgInput map[string]any,
	taskEnv common.EnvMap,
	cfg *Config,
) (*State, error) {
	env := cfg.GetEnv()
	id, err := cfg.LoadID()
	if err != nil {
		return nil, fmt.Errorf("failed to load agent ID: %w", err)
	}
	initializer := &state.AgentStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		AgentID:           id,
		ExecID:            execID,
		TaskExecID:        tExecID,
		WorkflowExecID:    wfExecID,
		TriggerInput:      tgInput,
		TaskEnv:           taskEnv,
		AgentEnv:          env,
	}
	bs, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent state: %w", err)
	}

	bsObj, ok := bs.(*state.BaseState)
	if !ok {
		return nil, fmt.Errorf("failed to convert to BaseState type, got %T", bs)
	}

	agState := &State{
		BaseState:      *bsObj,
		TaskExecID:     tExecID,
		WorkflowExecID: wfExecID,
	}

	return agState, nil
}
