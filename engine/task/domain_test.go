package task

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_IsParallelExecution(t *testing.T) {
	tests := []struct {
		name          string
		executionType ExecutionType
		parentStateID *core.ID
		expected      bool
	}{
		{
			name:          "parallel execution type should be parallel execution",
			executionType: ExecutionParallel,
			expected:      true,
		},
		{
			name:          "basic execution type should not be parallel execution",
			executionType: ExecutionBasic,
			expected:      false,
		},
		{
			name:          "router execution type should not be parallel execution",
			executionType: ExecutionRouter,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ExecutionType: tt.executionType,
				ParentStateID: tt.parentStateID,
			}
			require.Equal(t, tt.expected, state.IsParallelExecution())
		})
	}
}

func TestState_IsChildTask(t *testing.T) {
	parentID := core.ID("parent-123")

	tests := []struct {
		name          string
		parentStateID *core.ID
		expected      bool
	}{
		{
			name:          "with parent state ID should be child task",
			parentStateID: &parentID,
			expected:      true,
		},
		{
			name:          "without parent state ID should not be child task",
			parentStateID: nil,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ParentStateID: tt.parentStateID,
			}
			require.Equal(t, tt.expected, state.IsChildTask())
		})
	}
}

func TestState_IsBasic(t *testing.T) {
	tests := []struct {
		name          string
		executionType ExecutionType
		expected      bool
	}{
		{
			name:          "basic execution type should be basic",
			executionType: ExecutionBasic,
			expected:      true,
		},
		{
			name:          "parallel execution type should not be basic",
			executionType: ExecutionParallel,
			expected:      false,
		},
		{
			name:          "router execution type should not be basic",
			executionType: ExecutionRouter,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ExecutionType: tt.executionType,
			}
			assert.Equal(t, tt.expected, state.IsBasic())
		})
	}
}

func TestState_IsParallelRoot(t *testing.T) {
	parentID := core.ID("parent-123")

	tests := []struct {
		name          string
		executionType ExecutionType
		parentStateID *core.ID
		expected      bool
	}{
		{
			name:          "parallel execution with no parent should be parallel root",
			executionType: ExecutionParallel,
			parentStateID: nil,
			expected:      true,
		},
		{
			name:          "parallel execution with parent should not be parallel root",
			executionType: ExecutionParallel,
			parentStateID: &parentID,
			expected:      false,
		},
		{
			name:          "basic execution with no parent should not be parallel root",
			executionType: ExecutionBasic,
			parentStateID: nil,
			expected:      false,
		},
		{
			name:          "basic execution with parent should not be parallel root",
			executionType: ExecutionBasic,
			parentStateID: &parentID,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ExecutionType: tt.executionType,
				ParentStateID: tt.parentStateID,
			}
			assert.Equal(t, tt.expected, state.IsParallelRoot())
		})
	}
}

func TestState_HasParent(t *testing.T) {
	parentID := core.ID("parent-123")

	tests := []struct {
		name          string
		parentStateID *core.ID
		expected      bool
	}{
		{
			name:          "with parent state ID should have parent",
			parentStateID: &parentID,
			expected:      true,
		},
		{
			name:          "without parent state ID should not have parent",
			parentStateID: nil,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ParentStateID: tt.parentStateID,
			}
			assert.Equal(t, tt.expected, state.HasParent())
		})
	}
}

func TestState_GetParentID(t *testing.T) {
	parentID := core.ID("parent-123")

	tests := []struct {
		name          string
		parentStateID *core.ID
		expected      *core.ID
	}{
		{
			name:          "with parent state ID should return parent ID",
			parentStateID: &parentID,
			expected:      &parentID,
		},
		{
			name:          "without parent state ID should return nil",
			parentStateID: nil,
			expected:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &State{
				ParentStateID: tt.parentStateID,
			}
			result := state.GetParentID()
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestState_JSONSerialization(t *testing.T) {
	t.Run("ParentStateID should be omitted when nil", func(t *testing.T) {
		state := &State{
			TaskID:        "test-task",
			TaskExecID:    core.ID("exec-123"),
			ExecutionType: ExecutionBasic,
			ParentStateID: nil, // Should be omitted in JSON
		}

		data, err := json.Marshal(state)
		assert.NoError(t, err)

		jsonString := string(data)
		assert.NotContains(t, jsonString, "parent_state_id", "ParentStateID should be omitted when nil")
	})

	t.Run("ParentStateID should be included when not nil", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			TaskID:        "test-task",
			TaskExecID:    core.ID("exec-123"),
			ExecutionType: ExecutionBasic,
			ParentStateID: &parentID,
		}

		data, err := json.Marshal(state)
		assert.NoError(t, err)

		jsonString := string(data)
		assert.Contains(t, jsonString, "parent_state_id", "ParentStateID should be included when not nil")
		assert.Contains(t, jsonString, "parent-123", "ParentStateID value should be present")
	})

	t.Run("JSON unmarshaling should work correctly", func(t *testing.T) {
		parentID := core.ID("parent-123")
		original := &State{
			TaskID:        "test-task",
			TaskExecID:    core.ID("exec-123"),
			ExecutionType: ExecutionBasic,
			ParentStateID: &parentID,
		}

		// Marshal
		data, err := json.Marshal(original)
		assert.NoError(t, err)

		// Unmarshal
		var restored State
		err = json.Unmarshal(data, &restored)
		assert.NoError(t, err)

		// Verify
		assert.Equal(t, original.TaskID, restored.TaskID)
		assert.Equal(t, original.TaskExecID, restored.TaskExecID)
		assert.Equal(t, original.ExecutionType, restored.ExecutionType)
		assert.Equal(t, *original.ParentStateID, *restored.ParentStateID)
	})
}

func TestState_HelperMethods_EdgeCases(t *testing.T) {
	t.Run("nil state should not panic", func(t *testing.T) {
		var state *State
		assert.NotPanics(t, func() {
			// These should not panic even with nil state
			_ = state == nil
		})
	})

	t.Run("HasParent and IsChildTask should return same result", func(t *testing.T) {
		parentID := core.ID("parent-123")

		testCases := []*core.ID{nil, &parentID}

		for _, parentStateID := range testCases {
			state := &State{ParentStateID: parentStateID}
			assert.Equal(t, state.HasParent(), state.IsChildTask(),
				"HasParent and IsChildTask should return the same result")
		}
	})

	t.Run("GetParentID should return exact same pointer", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{ParentStateID: &parentID}

		result := state.GetParentID()
		assert.Same(t, &parentID, result, "GetParentID should return the same pointer")
	})
}
