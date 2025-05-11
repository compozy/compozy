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
	ID() StateID
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

// -----------------------------------------------------------------------------
// StateMap
// -----------------------------------------------------------------------------

type StateMap map[StateID]State

func (sm StateMap) Get(id StateID) (State, bool) {
	state, exists := sm[id]
	return state, exists
}

func (sm StateMap) Add(state State) {
	sm[state.ID()] = state
}

func (sm StateMap) Remove(id StateID) {
	delete(sm, id)
}

// -----------------------------------------------------------------------------
// StateID
// -----------------------------------------------------------------------------

type StateID struct {
	Component   common.ComponentType `json:"component"`
	ComponentID string               `json:"component_id"`
	ExecID      string               `json:"exec_id"`
}

func NewStateID(cfg common.Config, execID string) (*StateID, error) {
	id, err := cfg.LoadID()
	if err != nil {
		stID := &StateID{
			Component:   cfg.Component(),
			ComponentID: "",
			ExecID:      execID,
		}
		return stID, NewError(stID, "no_id", "no id found on config", err)
	}
	return &StateID{
		Component:   cfg.Component(),
		ComponentID: id,
		ExecID:      execID,
	}, nil
}

func (s *StateID) ToString() string {
	return fmt.Sprintf("%s:%s:%s", s.Component, s.ComponentID, s.ExecID)
}
