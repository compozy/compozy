package workflow

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/workflow"
)

type WorkflowState struct {
	id     string
	input  common.Input
	output *common.Output
	env    common.EnvMap
	Config *config.WorkflowConfig
}

func InitWorkflowState(execID string, input common.Input, cfg *config.WorkflowConfig) (*WorkflowState, error) {
	id, err := core.StateID(cfg, execID)
	if err != nil {
		return nil, core.NewWorkflowError(cfg.ID, "no_id", "no id found on config", err)
	}

	if err := cfg.ValidateParams(input); err != nil {
		return nil, core.NewWorkflowError(cfg.ID, "invalid_params", "invalid input params", err)
	}

	env, err := loadEnv(cfg.GetCWD(), cfg)
	if err != nil {
		return nil, err
	}

	// TODO: as_json() and parse_all() before save
	return &WorkflowState{
		id:     id,
		input:  input,
		output: nil,
		env:    env,
		Config: cfg,
	}, nil
}

func loadEnv(cwd string, cfg *config.WorkflowConfig) (common.EnvMap, error) {
	env, err := common.NewEnvFromFile(cwd)
	if err != nil {
		return nil, core.NewWorkflowError(cfg.ID, "env_read_fail", "failed to read env file", err)
	}

	env, err = env.Merge(cfg.Env)
	if err != nil {
		return nil, core.NewWorkflowError(cfg.ID, "env_merge_fail", "failed to merge environment variables", err)
	}

	return env, nil
}

func (ws *WorkflowState) ID() string {
	return ws.id
}

func (ws *WorkflowState) Env() *common.EnvMap {
	return &ws.env
}

func (ws *WorkflowState) Input() *common.Input {
	return &ws.input
}

func (ws *WorkflowState) Output() *common.Output {
	return ws.output
}

func (ws *WorkflowState) FromParentState(parent core.State) error {
	return core.FromParentState(ws, parent)
}

func (ws *WorkflowState) WithEnv(env common.EnvMap) error {
	newEnv, err := core.WithEnv(ws, env)
	if err != nil {
		return core.NewWorkflowError(ws.Config.ID, "merge_env_fail", "failed to merge env", err)
	}
	ws.env = *newEnv
	return nil
}

func (ws *WorkflowState) WithInput(input common.Input) error {
	newInput, err := core.WithInput(ws, input)
	if err != nil {
		return core.NewWorkflowError(ws.Config.ID, "merge_input_fail", "failed to merge input", err)
	}
	ws.input = *newInput
	return nil
}
