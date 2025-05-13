package task

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/task"
)

type State struct {
	id     *core.StateID
	env    *common.EnvMap
	input  *common.Input
	output *common.Output
	Config *config.Config
}

func InitTaskState(stID *core.StateID, cfg *config.Config, parent core.State) (*State, error) {
	if err := cfg.ValidateParams(*cfg.With); err != nil {
		errResponse := core.NewError(stID, "invalid_params", "invalid input params", err)
		return nil, &errResponse
	}

	state := &State{
		id:     stID,
		input:  cfg.With,
		env:    &cfg.Env,
		output: nil,
		Config: cfg,
	}

	if parent.Env() != nil {
		if err := state.WithEnv(*parent.Env()); err != nil {
			return nil, err
		}
	}

	// TODO: as_json() and parse_all() before save
	return state, nil
}

func (ts *State) ID() core.StateID {
	return *ts.id
}

func (ts *State) Env() *common.EnvMap {
	return ts.env
}

func (ts *State) Input() *common.Input {
	return ts.input
}

func (ts *State) Output() *common.Output {
	return ts.output
}

func (ts *State) FromParentState(parent core.State) error {
	return core.FromParentState(ts, parent)
}

func (ts *State) WithEnv(env common.EnvMap) error {
	newEnv, err := core.WithEnv(ts, env)
	if err != nil {
		errResponse := core.NewError(ts.id, "merge_env_fail", "failed to merge env", err)
		return &errResponse
	}
	ts.env = newEnv
	return nil
}

func (ts *State) WithInput(input common.Input) error {
	newInput, err := core.WithInput(ts, input)
	if err != nil {
		errResponse := core.NewError(ts.id, "merge_input_fail", "failed to merge input", err)
		return &errResponse
	}
	ts.input = newInput
	return nil
}
