package core

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
	"github.com/compozy/compozy/internal/parser/common"
)

// -----------------------------------------------------------------------------
// State Interface
// -----------------------------------------------------------------------------

type State interface {
	ID() string
	Env() *common.EnvMap
	Input() *common.Input
	Output() *common.Output
	FromParentState(parent State) error
	WithEnv(env common.EnvMap) error
	WithInput(input common.Input) error
}

// -----------------------------------------------------------------------------
// State Helper Functions
// -----------------------------------------------------------------------------

func FromParentState(state State, parent State) error {
	if parent == nil {
		return nil
	}

	if parent.Env() != nil {
		env, err := WithEnv(state, *parent.Env())
		if err != nil {
			return fmt.Errorf("failed to merge env: %w", err)
		}
		state.WithEnv(*env)
	}

	if parent.Input() != nil {
		input, err := WithInput(state, *parent.Input())
		if err != nil {
			return fmt.Errorf("failed to merge input: %w", err)
		}
		state.WithInput(*input)
	}

	return nil
}

func WithEnv(state State, env common.EnvMap) (*common.EnvMap, error) {
	dst := make(common.EnvMap)
	if state.Env() != nil {
		maps.Copy(dst, *state.Env())
	}
	if err := mergo.Merge(&dst, env, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	return &dst, nil
}

func WithInput(state State, input common.Input) (*common.Input, error) {
	dst := make(common.Input)
	if state.Input() != nil {
		maps.Copy(dst, *state.Input())
	}
	if err := mergo.Merge(&dst, input, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	return &dst, nil
}

func StateID(cfg common.Config, execID string) (string, error) {
	id, err := cfg.LoadID()
	if err != nil {
		return "", fmt.Errorf("failed to load config id: %w", err)
	}
	return fmt.Sprintf("%s:%s:%s", cfg.Component(), id, execID), nil
}

// -----------------------------------------------------------------------------
// StateMap
// -----------------------------------------------------------------------------

type StateMap map[string]State

func (sm StateMap) Get(id string) (State, bool) {
	state, exists := sm[id]
	return state, exists
}

func (sm StateMap) Add(state State) {
	sm[state.ID()] = state
}

func (sm StateMap) Remove(id string) {
	delete(sm, id)
}
