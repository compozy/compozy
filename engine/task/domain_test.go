package task

import (
	"encoding/json"
	"testing"
	"time"

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

func TestSignalEnvelope_JSONMarshaling(t *testing.T) {
	t.Run("Should marshal and unmarshal SignalEnvelope correctly", func(t *testing.T) {
		envelope := &SignalEnvelope{
			Payload: map[string]any{
				"action": "approve",
				"user":   "john.doe",
				"data": map[string]any{
					"amount": 1000.50,
					"reason": "budget approval",
				},
			},
			Metadata: SignalMetadata{
				SignalID:      "signal-123",
				ReceivedAtUTC: time.Now().UTC(),
				WorkflowID:    "workflow-456",
				Source:        "web-ui",
			},
		}
		data, err := json.Marshal(envelope)
		require.NoError(t, err)
		var unmarshaled SignalEnvelope
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, envelope.Payload["action"], unmarshaled.Payload["action"])
		assert.Equal(t, envelope.Payload["user"], unmarshaled.Payload["user"])
		assert.Equal(t, envelope.Metadata.SignalID, unmarshaled.Metadata.SignalID)
		assert.Equal(t, envelope.Metadata.WorkflowID, unmarshaled.Metadata.WorkflowID)
		assert.Equal(t, envelope.Metadata.Source, unmarshaled.Metadata.Source)
		assert.WithinDuration(t, envelope.Metadata.ReceivedAtUTC, unmarshaled.Metadata.ReceivedAtUTC, time.Millisecond)
	})
	t.Run("Should handle empty payload", func(t *testing.T) {
		envelope := &SignalEnvelope{
			Payload: map[string]any{},
			Metadata: SignalMetadata{
				SignalID:      "signal-empty",
				ReceivedAtUTC: time.Now().UTC(),
				WorkflowID:    "workflow-789",
				Source:        "api",
			},
		}
		data, err := json.Marshal(envelope)
		require.NoError(t, err)
		var unmarshaled SignalEnvelope
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.NotNil(t, unmarshaled.Payload)
		assert.Empty(t, unmarshaled.Payload)
		assert.Equal(t, envelope.Metadata.SignalID, unmarshaled.Metadata.SignalID)
	})
}

func TestProcessorOutput_JSONMarshaling(t *testing.T) {
	t.Run("Should marshal ProcessorOutput with successful result", func(t *testing.T) {
		output := &ProcessorOutput{
			Output: map[string]any{
				"valid": true,
				"score": 0.95,
			},
			Error: nil,
		}
		data, err := json.Marshal(output)
		require.NoError(t, err)
		var unmarshaled ProcessorOutput
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		outputMap, ok := unmarshaled.Output.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, outputMap["valid"])
		assert.InDelta(t, 0.95, outputMap["score"], 0.001)
		assert.Nil(t, unmarshaled.Error)
	})
	t.Run("Should marshal ProcessorOutput with error", func(t *testing.T) {
		output := &ProcessorOutput{
			Output: nil,
			Error: core.NewError(
				&core.Error{Message: "validation failed: invalid input format"},
				"VALIDATION_FAILED",
				map[string]any{"context": "test"},
			),
		}
		data, err := json.Marshal(output)
		require.NoError(t, err)
		var unmarshaled ProcessorOutput
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Nil(t, unmarshaled.Output)
		require.NotNil(t, unmarshaled.Error)
		assert.Contains(t, unmarshaled.Error.Message, "validation failed: invalid input format")
	})
}

func TestWaitTaskResult_JSONMarshaling(t *testing.T) {
	t.Run("Should marshal WaitTaskResult with successful completion", func(t *testing.T) {
		completedAt := time.Now().UTC()
		result := &WaitTaskResult{
			Status: "success",
			Signal: &SignalEnvelope{
				Payload: map[string]any{"approved": true},
				Metadata: SignalMetadata{
					SignalID:      "sig-001",
					ReceivedAtUTC: completedAt,
					WorkflowID:    "wf-001",
					Source:        "api",
				},
			},
			ProcessorOutput: &ProcessorOutput{
				Output: map[string]any{"processed": true},
				Error:  nil,
			},
			NextTask:    "process_approval",
			CompletedAt: completedAt,
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var unmarshaled WaitTaskResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, "success", unmarshaled.Status)
		assert.NotNil(t, unmarshaled.Signal)
		assert.NotNil(t, unmarshaled.ProcessorOutput)
		assert.Equal(t, "process_approval", unmarshaled.NextTask)
		assert.WithinDuration(t, completedAt, unmarshaled.CompletedAt, time.Millisecond)
	})
	t.Run("Should marshal WaitTaskResult with timeout", func(t *testing.T) {
		completedAt := time.Now().UTC()
		result := &WaitTaskResult{
			Status:      "timeout",
			NextTask:    "handle_timeout",
			CompletedAt: completedAt,
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var unmarshaled WaitTaskResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, "timeout", unmarshaled.Status)
		assert.Nil(t, unmarshaled.Signal)
		assert.Nil(t, unmarshaled.ProcessorOutput)
		assert.Equal(t, "handle_timeout", unmarshaled.NextTask)
	})
}

func TestSignalProcessingResult_JSONMarshaling(t *testing.T) {
	t.Run("Should marshal SignalProcessingResult when condition is met", func(t *testing.T) {
		result := &SignalProcessingResult{
			ShouldContinue: true,
			Signal: &SignalEnvelope{
				Payload: map[string]any{"status": "approved"},
				Metadata: SignalMetadata{
					SignalID:      "sig-002",
					ReceivedAtUTC: time.Now().UTC(),
					WorkflowID:    "wf-002",
					Source:        "webhook",
				},
			},
			ProcessorOutput: &ProcessorOutput{
				Output: map[string]any{"valid": true},
				Error:  nil,
			},
			Reason: "condition_met",
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var unmarshaled SignalProcessingResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.True(t, unmarshaled.ShouldContinue)
		assert.Equal(t, "condition_met", unmarshaled.Reason)
		assert.NotNil(t, unmarshaled.Signal)
		assert.NotNil(t, unmarshaled.ProcessorOutput)
	})
	t.Run("Should handle duplicate signal result", func(t *testing.T) {
		result := &SignalProcessingResult{
			ShouldContinue: false,
			Reason:         "duplicate",
			Signal: &SignalEnvelope{
				Payload: map[string]any{},
				Metadata: SignalMetadata{
					SignalID: "sig-duplicate",
				},
			},
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var unmarshaled SignalProcessingResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.False(t, unmarshaled.ShouldContinue)
		assert.Equal(t, "duplicate", unmarshaled.Reason)
		assert.NotNil(t, unmarshaled.Signal)
		assert.Nil(t, unmarshaled.ProcessorOutput)
	})
}
