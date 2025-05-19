package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/state"
)

type State struct {
	state.BaseState
	Tasks state.Map `json:"tasks,omitempty"`
}

func NewWorkflowState(cID string, triggerInput map[string]any, projectEnv common.EnvMap, cfg *Config) (*State, error) {
	env := cfg.GetEnv()
	id, err := cfg.LoadID()
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow ID: %w", err)
	}
	initializer := &state.WorkflowStateInitializer{
		CommonInitializer: state.NewCommonInitializer(),
		WorkflowID:        id,
		ExecID:            cID,
		TriggerInput:      triggerInput,
		ProjectEnv:        projectEnv,
		WorkflowEnv:       env,
	}
	bs, err := initializer.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize workflow state: %w", err)
	}

	bsObj, ok := bs.(*state.BaseState)
	if !ok {
		return nil, fmt.Errorf("failed to convert to BaseState type, got %T", bs)
	}

	state := &State{
		BaseState: *bsObj,
		Tasks:     make(state.Map),
	}

	return state, nil
}
