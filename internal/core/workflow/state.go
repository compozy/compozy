package workflow

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/workflow"
)

type State struct {
	id     *core.StateID
	input  common.Input
	output *common.Output
	env    common.EnvMap
	Config *config.Config
}

func InitWorkflowState(stID *core.StateID, input common.Input, cfg *config.Config) (*State, error) {
	env, err := loadEnv(stID, cfg.Env, cfg.GetCWD())
	if err != nil {
		return nil, err
	}

	// TODO: as_json() and parse_all() before save
	return &State{
		id:     stID,
		input:  input,
		output: nil,
		env:    env,
		Config: cfg,
	}, nil
}

func loadEnv(stID *core.StateID, currEnv common.EnvMap, cwd *common.CWD) (common.EnvMap, error) {
	env, err := common.NewEnvFromFile(cwd.PathStr())
	if err != nil {
		return nil, core.NewError(stID, "env_read_fail", "failed to read env file", err)
	}

	env, err = currEnv.Merge(env)
	if err != nil {
		return nil, core.NewError(stID, "env_merge_fail", "failed to merge environment variables", err)
	}

	return env, nil
}

func (ws *State) ID() core.StateID {
	return *ws.id
}

func (ws *State) Env() *common.EnvMap {
	return &ws.env
}

func (ws *State) Input() *common.Input {
	return &ws.input
}

func (ws *State) Output() *common.Output {
	return ws.output
}

func (ws *State) FromParentState(parent core.State) error {
	return core.FromParentState(ws, parent)
}

func (ws *State) WithEnv(env common.EnvMap) error {
	newEnv, err := core.WithEnv(ws, env)
	if err != nil {
		return core.NewError(ws.id, "merge_env_fail", "failed to merge env", err)
	}
	ws.env = *newEnv
	return nil
}

func (ws *State) WithInput(input common.Input) error {
	newInput, err := core.WithInput(ws, input)
	if err != nil {
		return core.NewError(ws.id, "merge_input_fail", "failed to merge input", err)
	}
	ws.input = *newInput
	return nil
}
