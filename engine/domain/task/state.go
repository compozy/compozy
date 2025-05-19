package task

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/state"
)

type State struct {
	state.BaseState
	WorkflowExecID string `json:"workflow_exec_id"`
}

func NewTaskState(execID, wExecID string, triggerInput map[string]any, workflowEnv common.EnvMap, cfg *Config) (*State, error) {
	env := cfg.GetEnv()
	id, err := cfg.LoadID()
	if err != nil {
		return nil, fmt.Errorf("failed to load task ID: %w", err)
	}
	initializer := &state.TaskStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		TaskID:            id,
		ExecID:            execID,
		WorkflowExecID:    wExecID,
		TriggerInput:      triggerInput,
		WorkflowEnv:       workflowEnv,
		TaskEnv:           env,
	}
	bs, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize task state: %w", err)
	}

	bsObj, ok := bs.(*state.BaseState)
	if !ok {
		return nil, fmt.Errorf("failed to convert to BaseState type, got %T", bs)
	}

	state := &State{
		BaseState:      *bsObj,
		WorkflowExecID: wExecID,
	}

	return state, nil
}
