package state

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/common"
)

// -----------------------------------------------------------------------------
// State Helper Functions
// -----------------------------------------------------------------------------

func DefFromParent(state State, parent State) error {
	if parent == nil {
		return nil
	}

	if parent.GetEnv() != nil {
		env, err := DefWithEnv(state, *parent.GetEnv())
		if err != nil {
			return fmt.Errorf("failed to merge env: %w", err)
		}
		if err := state.WithEnv(*env); err != nil {
			return fmt.Errorf("failed to set env: %w", err)
		}
	}

	if parent.GetInput() != nil {
		input, err := DefWithInput(state, *parent.GetInput())
		if err != nil {
			return fmt.Errorf("failed to merge input: %w", err)
		}
		if err := state.WithInput(*input); err != nil {
			return fmt.Errorf("failed to set input: %w", err)
		}
	}

	return nil
}

func DefWithEnv(state State, env common.EnvMap) (*common.EnvMap, error) {
	dst := make(common.EnvMap)
	if state.GetEnv() != nil {
		maps.Copy(dst, *state.GetEnv())
	}
	if err := mergo.Merge(&dst, env, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	return &dst, nil
}

func DefWithInput(state State, input common.Input) (*common.Input, error) {
	dst := make(common.Input)
	if state.GetInput() != nil {
		maps.Copy(dst, *state.GetInput())
	}
	if err := mergo.Merge(&dst, input, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge input: %w", err)
	}
	return &dst, nil
}
