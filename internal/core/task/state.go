package task

import (
	"github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/parser/common"
	config "github.com/compozy/compozy/internal/parser/task"
)

type TaskState struct {
	id     string
	env    *common.EnvMap
	input  *common.Input
	output *common.Output
	Config *config.TaskConfig
}

func InitTaskState(execID string, cfg *config.TaskConfig, parent core.State) (*TaskState, error) {
	id, err := core.StateID(cfg, execID)
	if err != nil {
		return nil, core.NewTaskError(cfg.ID, "no_id", "no id found on config", err)
	}

	if err := cfg.ValidateParams(*cfg.With); err != nil {
		return nil, core.NewTaskError(cfg.ID, "invalid_params", "invalid input params", err)
	}

	state := &TaskState{
		id:     id,
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

func (ts *TaskState) ID() string {
	return ts.id
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
		return core.NewTaskError(ts.id, "merge_env_fail", "failed to merge env", err)
	}
	ts.env = newEnv
	return nil
}

func (ts *TaskState) WithInput(input common.Input) error {
	newInput, err := core.WithInput(ts, input)
	if err != nil {
		return core.NewTaskError(ts.id, "merge_input_fail", "failed to merge input", err)
	}
	ts.input = newInput
	return nil
}
