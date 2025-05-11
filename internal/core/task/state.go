package task

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/task"
)

type TaskState struct {
	id     *core.StateID
	env    *common.EnvMap
	input  *common.Input
	output *common.Output
	Config *config.TaskConfig
}

func InitTaskState(stID *core.StateID, cfg *config.TaskConfig, parent core.State) (*TaskState, error) {
	if err := cfg.ValidateParams(*cfg.With); err != nil {
		return nil, core.NewError(stID, "invalid_params", "invalid input params", err)
	}

	state := &TaskState{
		id:     stID,
		input:  cfg.With,
		env:    &cfg.Env,
		output: nil,
		Config: cfg,
	}

	if parent.Env() != nil {
		state.WithEnv(*parent.Env())
	}

	// TODO: as_json() and parse_all() before save
	return state, nil
}

func (ts *TaskState) ID() core.StateID {
	return *ts.id
}

func (ts *TaskState) Env() *common.EnvMap {
	return ts.env
}

func (ts *TaskState) Input() *common.Input {
	return ts.input
}

func (ts *TaskState) Output() *common.Output {
	return ts.output
}

func (ts *TaskState) FromParentState(parent core.State) error {
	return core.FromParentState(ts, parent)
}

func (ts *TaskState) WithEnv(env common.EnvMap) error {
	newEnv, err := core.WithEnv(ts, env)
	if err != nil {
		return core.NewError(ts.id, "merge_env_fail", "failed to merge env", err)
	}
	ts.env = newEnv
	return nil
}

func (ts *TaskState) WithInput(input common.Input) error {
	newInput, err := core.WithInput(ts, input)
	if err != nil {
		return core.NewError(ts.id, "merge_input_fail", "failed to merge input", err)
	}
	ts.input = newInput
	return nil
}
