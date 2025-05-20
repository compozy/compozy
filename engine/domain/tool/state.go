package tool

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

func NewToolState(execID, tExecID, wfExecID string, tgInput map[string]any, taskEnv common.EnvMap, cfg *Config) (*State, error) {
	env := cfg.GetEnv()
	id, err := cfg.LoadID()
	if err != nil {
		return nil, fmt.Errorf("failed to load tool ID: %w", err)
	}
	initializer := &state.ToolStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		ToolID:            id,
		ExecID:            execID,
		TaskExecID:        tExecID,
		WorkflowExecID:    wfExecID,
		TriggerInput:      tgInput,
		TaskEnv:           taskEnv,
		ToolEnv:           env,
	}
	bs, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tool state: %w", err)
	}

	bsObj, ok := bs.(*state.BaseState)
	if !ok {
		return nil, fmt.Errorf("failed to convert to BaseState type, got %T", bs)
	}

	state := &State{
		BaseState:      *bsObj,
		TaskExecID:     tExecID,
		WorkflowExecID: wfExecID,
	}

	return state, nil
}
