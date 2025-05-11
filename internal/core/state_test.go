package core

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockState is a concrete implementation of the State interface for testing
type MockState struct {
	id     string
	env    *common.EnvMap
	input  *common.Input
	output *common.Output
}

func (m *MockState) ID() string                         { return m.id }
func (m *MockState) Env() *common.EnvMap                { return m.env }
func (m *MockState) Input() *common.Input               { return m.input }
func (m *MockState) Output() *common.Output             { return m.output }
func (m *MockState) FromParentState(parent State) error { return FromParentState(m, parent) }
func (m *MockState) WithEnv(env common.EnvMap) error {
	mergedEnv, err := WithEnv(m, env)
	if err != nil {
		return err
	}
	m.env = mergedEnv
	return nil
}
func (m *MockState) WithInput(input common.Input) error {
	mergedInput, err := WithInput(m, input)
	if err != nil {
		return err
	}
	m.input = mergedInput
	return nil
}

func TestState(t *testing.T) {
	t.Run("FromParentState should merge parent state into state", func(t *testing.T) {
		// Setup parent and state
		parentEnv := common.EnvMap{"parent-key": "parent-value", "shared": "parent-shared"}
		parentInput := common.Input{"parent-input": "parent-value", "shared-input": "parent-shared"}
		parent := &MockState{
			id:    "parent-id",
			env:   &parentEnv,
			input: &parentInput,
		}

		stateEnv := common.EnvMap{"state-key": "state-value", "shared": "state-shared"}
		stateInput := common.Input{"state-input": "state-value", "shared-input": "state-shared"}
		state := &MockState{
			id:    "state-id",
			env:   &stateEnv,
			input: &stateInput,
		}

		// Perform merge
		err := FromParentState(state, parent)
		require.NoError(t, err)

		// Verify results
		assert.Equal(t, "state-id", state.ID(), "ID should be preserved from state")
		// Env: Expect parent env to override state env
		assert.Equal(t, "parent-value", (*state.Env())["parent-key"], "Parent env key should be merged")
		assert.Equal(t, "parent-shared", (*state.Env())["shared"], "Shared env key should be overridden by parent")
		assert.Equal(t, "state-value", (*state.Env())["state-key"], "State env key should be preserved")
		// Input: Expect parent input to override state input
		assert.Equal(t, "parent-value", (*state.Input())["parent-input"], "Parent input key should be merged")
		assert.Equal(t, "parent-shared", (*state.Input())["shared-input"], "Shared input key should be overridden by parent")
		assert.Equal(t, "state-value", (*state.Input())["state-input"], "State input key should be preserved")
	})

	t.Run("WithEnv should merge environment map", func(t *testing.T) {
		stateEnv := common.EnvMap{"state-key": "state-value", "shared": "state-shared"}
		state := &MockState{
			env: &stateEnv,
		}

		newEnv := common.EnvMap{"new-key": "new-value", "shared": "new-shared"}

		err := state.WithEnv(newEnv)
		require.NoError(t, err)

		assert.Equal(t, "state-value", (*state.Env())["state-key"], "State env key should be preserved")
		assert.Equal(t, "new-value", (*state.Env())["new-key"], "New env key should be merged")
		assert.Equal(t, "new-shared", (*state.Env())["shared"], "Shared env key should be overridden by new env")
	})

	t.Run("WithInput should merge input", func(t *testing.T) {
		stateInput := common.Input{"state-input": "state-value", "shared-input": "state-shared"}
		state := &MockState{
			input: &stateInput,
		}

		newInput := common.Input{"new-input": "new-value", "shared-input": "new-shared"}

		err := state.WithInput(newInput)
		require.NoError(t, err)

		assert.Equal(t, "state-value", (*state.Input())["state-input"], "State input key should be preserved")
		assert.Equal(t, "new-value", (*state.Input())["new-input"], "New input key should be merged")
		assert.Equal(t, "new-shared", (*state.Input())["shared-input"], "Shared input key should be overridden by new input")
	})

	t.Run("StateMap should handle add, get, and remove", func(t *testing.T) {
		sm := make(StateMap)

		// Create a state
		stateEnv := common.EnvMap{"key": "value"}
		state := &MockState{
			id:  "state1",
			env: &stateEnv,
		}

		// Test Add
		sm.Add(state)
		retrieved, exists := sm.Get("state1")
		require.True(t, exists, "State should exist in map")
		assert.Equal(t, state, retrieved, "Retrieved state should match added state")

		// Test Get for non-existent state
		_, exists = sm.Get("non-existent")
		assert.False(t, exists, "Non-existent state should not be found")

		// Test Remove
		sm.Remove("state1")
		_, exists = sm.Get("state1")
		assert.False(t, exists, "State should be removed from map")
	})

	t.Run("FromParentState should handle nil parent", func(t *testing.T) {
		stateEnv := common.EnvMap{"key": "value"}
		state := &MockState{
			id:  "state-id",
			env: &stateEnv,
		}

		err := FromParentState(state, nil)
		assert.NoError(t, err, "Should not error on nil parent")
		assert.Equal(t, "value", (*state.Env())["key"], "State env should be unchanged")
	})

	t.Run("WithEnv should handle nil env", func(t *testing.T) {
		stateEnv := common.EnvMap{"key": "value"}
		state := &MockState{
			env: &stateEnv,
		}

		var nilEnv common.EnvMap
		mergedEnv, err := WithEnv(state, nilEnv)
		require.NoError(t, err)
		assert.Equal(t, stateEnv, *mergedEnv, "Env should be unchanged")
	})

	t.Run("WithInput should handle nil input", func(t *testing.T) {
		stateInput := common.Input{"key": "value"}
		state := &MockState{
			input: &stateInput,
		}

		var nilInput common.Input
		mergedInput, err := WithInput(state, nilInput)
		require.NoError(t, err)
		assert.Equal(t, stateInput, *mergedInput, "Input should be unchanged")
	})

	t.Run("FromParentState should handle nil on current but not on parent", func(t *testing.T) {
		parentEnv := common.EnvMap{"key": "value"}
		parentInput := common.Input{"input-key": "input-value"}
		parent := &MockState{
			env:   &parentEnv,
			input: &parentInput,
		}

		state := &MockState{
			id:    "state-id",
			env:   nil,
			input: nil,
		}

		err := FromParentState(state, parent)
		assert.NoError(t, err, "Should not error on nil current")
		assert.Equal(t, "value", (*state.Env())["key"], "State env should be set from parent")
		assert.Equal(t, "input-value", (*state.Input())["input-key"], "State input should be set from parent")
	})

	t.Run("FromParentState should handle nil input on current but not on parent", func(t *testing.T) {
		parentInput := common.Input{"key": "value"}
		parent := &MockState{
			input: &parentInput,
		}

		state := &MockState{
			id:    "state-id",
			input: nil,
		}

		err := FromParentState(state, parent)
		assert.NoError(t, err, "Should not error on nil current input")
		assert.Equal(t, "value", (*state.Input())["key"], "State input should be set from parent")
	})
}
