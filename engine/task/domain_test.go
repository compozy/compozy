package task

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_IsParallelExecution(t *testing.T) {
	t.Run("Should return true for parallel execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionParallel,
		}
		require.True(t, state.IsParallelExecution())
	})

	t.Run("Should return false for basic execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionBasic,
		}
		require.False(t, state.IsParallelExecution())
	})

	t.Run("Should return false for router execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionRouter,
		}
		require.False(t, state.IsParallelExecution())
	})
}

func TestState_IsChildTask(t *testing.T) {
	t.Run("Should return true when has parent state ID", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			ParentStateID: &parentID,
		}
		require.True(t, state.IsChildTask())
	})

	t.Run("Should return false when no parent state ID", func(t *testing.T) {
		state := &State{
			ParentStateID: nil,
		}
		require.False(t, state.IsChildTask())
	})
}

func TestState_IsBasic(t *testing.T) {
	t.Run("Should return true for basic execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionBasic,
		}
		assert.True(t, state.IsBasic())
	})

	t.Run("Should return false for parallel execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionParallel,
		}
		assert.False(t, state.IsBasic())
	})

	t.Run("Should return false for router execution type", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionRouter,
		}
		assert.False(t, state.IsBasic())
	})
}

func TestState_IsParallelRoot(t *testing.T) {
	t.Run("Should return true for parallel execution with no parent", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionParallel,
			ParentStateID: nil,
		}
		assert.True(t, state.IsParallelRoot())
	})

	t.Run("Should return false for parallel execution with parent", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			ExecutionType: ExecutionParallel,
			ParentStateID: &parentID,
		}
		assert.False(t, state.IsParallelRoot())
	})

	t.Run("Should return false for basic execution with no parent", func(t *testing.T) {
		state := &State{
			ExecutionType: ExecutionBasic,
			ParentStateID: nil,
		}
		assert.False(t, state.IsParallelRoot())
	})

	t.Run("Should return false for basic execution with parent", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			ExecutionType: ExecutionBasic,
			ParentStateID: &parentID,
		}
		assert.False(t, state.IsParallelRoot())
	})
}

func TestState_HasParent(t *testing.T) {
	t.Run("Should return true when has parent state ID", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			ParentStateID: &parentID,
		}
		assert.True(t, state.HasParent())
	})

	t.Run("Should return false when no parent state ID", func(t *testing.T) {
		state := &State{
			ParentStateID: nil,
		}
		assert.False(t, state.HasParent())
	})
}

func TestState_GetParentID(t *testing.T) {
	t.Run("Should return parent ID when has parent state ID", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{
			ParentStateID: &parentID,
		}
		result := state.GetParentID()
		assert.Equal(t, parentID, *result)
	})

	t.Run("Should return nil when no parent state ID", func(t *testing.T) {
		state := &State{
			ParentStateID: nil,
		}
		result := state.GetParentID()
		assert.Nil(t, result)
	})
}

func TestState_JSONSerialization(t *testing.T) {
	t.Run("Should omit ParentStateID when nil", func(t *testing.T) {
		state := &State{
			TaskID:        "test-task",
			TaskExecID:    core.ID("exec-123"),
			ExecutionType: ExecutionBasic,
			ParentStateID: nil,
		}
		data, err := json.Marshal(state)
		assert.NoError(t, err)
		jsonString := string(data)
		assert.NotContains(t, jsonString, "parent_state_id", "ParentStateID should be omitted when nil")
	})

	t.Run("Should include ParentStateID when not nil", func(t *testing.T) {
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

	t.Run("Should unmarshal JSON correctly", func(t *testing.T) {
		parentID := core.ID("parent-123")
		original := &State{
			TaskID:        "test-task",
			TaskExecID:    core.ID("exec-123"),
			ExecutionType: ExecutionBasic,
			ParentStateID: &parentID,
		}
		data, err := json.Marshal(original)
		assert.NoError(t, err)
		var restored State
		err = json.Unmarshal(data, &restored)
		assert.NoError(t, err)
		assert.Equal(t, original.TaskID, restored.TaskID)
		assert.Equal(t, original.TaskExecID, restored.TaskExecID)
		assert.Equal(t, original.ExecutionType, restored.ExecutionType)
		assert.Equal(t, *original.ParentStateID, *restored.ParentStateID)
	})
}

func TestState_HelperMethods_EdgeCases(t *testing.T) {
	t.Run("Should not panic with nil state", func(t *testing.T) {
		var state *State
		assert.NotPanics(t, func() {
			_ = state == nil
		})
	})

	t.Run("Should return same result for HasParent and IsChildTask", func(t *testing.T) {
		parentID := core.ID("parent-123")
		testCases := []*core.ID{nil, &parentID}
		for _, parentStateID := range testCases {
			state := &State{ParentStateID: parentStateID}
			assert.Equal(t, state.HasParent(), state.IsChildTask(),
				"HasParent and IsChildTask should return the same result")
		}
	})

	t.Run("Should return exact same pointer from GetParentID", func(t *testing.T) {
		parentID := core.ID("parent-123")
		state := &State{ParentStateID: &parentID}
		result := state.GetParentID()
		assert.Same(t, &parentID, result, "GetParentID should return the same pointer")
	})
}
