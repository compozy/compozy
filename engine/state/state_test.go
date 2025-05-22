package state

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID_String(t *testing.T) {
	corrID, err := common.NewID()
	require.NoError(t, err)
	execID, err := common.NewID()
	require.NoError(t, err)
	compType := nats.ComponentWorkflow
	id := NewID(compType, corrID, execID)
	expected := string(compType) + "_" + corrID.String() + execID.String()
	assert.Equal(t, expected, id.String())
}

func TestIDFromString(t *testing.T) {
	corrID, err := common.NewID()
	require.NoError(t, err)
	execID, err := common.NewID()
	require.NoError(t, err)

	t.Run("Should successfully parse valid workflow ID", func(t *testing.T) {
		original := NewID(nats.ComponentWorkflow, corrID, execID)
		idStr := original.String()
		parsed, err := IDFromString(idStr)
		require.NoError(t, err)
		assert.Equal(t, original, parsed)
	})

	t.Run("Should successfully parse valid task ID", func(t *testing.T) {
		original := NewID(nats.ComponentTask, corrID, execID)
		idStr := original.String()
		parsed, err := IDFromString(idStr)
		require.NoError(t, err)
		assert.Equal(t, original, parsed)
	})

	t.Run("Should successfully parse valid agent ID", func(t *testing.T) {
		original := NewID(nats.ComponentAgent, corrID, execID)
		idStr := original.String()
		parsed, err := IDFromString(idStr)
		require.NoError(t, err)
		assert.Equal(t, original, parsed)
	})

	t.Run("Should successfully parse valid tool ID", func(t *testing.T) {
		original := NewID(nats.ComponentTool, corrID, execID)
		idStr := original.String()
		parsed, err := IDFromString(idStr)
		require.NoError(t, err)
		assert.Equal(t, original, parsed)
	})

	t.Run("Should return error for empty string", func(t *testing.T) {
		_, err := IDFromString("")
		assert.Error(t, err)
	})

	t.Run("Should return error for string without underscore", func(t *testing.T) {
		_, err := IDFromString("workflow" + corrID.String() + execID.String())
		assert.Error(t, err)
	})

	t.Run("Should return error for string with too many parts", func(t *testing.T) {
		_, err := IDFromString("workflow_123_456")
		assert.Error(t, err)
	})

	t.Run("Should return error for ID part too short", func(t *testing.T) {
		_, err := IDFromString("workflow_123456")
		assert.Error(t, err)
	})

	t.Run("Should return error for truncated ID part", func(t *testing.T) {
		_, err := IDFromString("workflow_" + corrID.String() + execID.String()[:10])
		assert.Error(t, err)
	})
}

func TestBaseState_Getters(t *testing.T) {
	corrID, err := common.NewID()
	require.NoError(t, err)

	execID, err := common.NewID()
	require.NoError(t, err)

	stateID := NewID(nats.ComponentWorkflow, corrID, execID)

	input := &common.Input{"key": "value"}
	output := &common.Output{"result": "success"}
	env := &common.EnvMap{"VAR": "value"}
	trigger := &common.Input{"event": "started"}
	testErr := &Error{Message: "error message", Code: "ERR_CODE"}

	baseState := &BaseState{
		StateID: stateID,
		Status:  nats.StatusRunning,
		Trigger: trigger,
		Input:   input,
		Output:  output,
		Env:     env,
		Error:   testErr,
	}

	t.Run("Should correctly return ID", func(t *testing.T) {
		assert.Equal(t, stateID, baseState.GetID())
	})

	t.Run("Should correctly return correlation ID", func(t *testing.T) {
		assert.Equal(t, corrID, baseState.GetCorrelationID())
	})

	t.Run("Should correctly return execution ID", func(t *testing.T) {
		assert.Equal(t, execID, baseState.GetExecID())
	})

	t.Run("Should correctly return status", func(t *testing.T) {
		assert.Equal(t, nats.StatusRunning, baseState.GetStatus())
	})

	t.Run("Should correctly return environment", func(t *testing.T) {
		assert.Equal(t, env, baseState.GetEnv())
	})

	t.Run("Should correctly return trigger", func(t *testing.T) {
		assert.Equal(t, trigger, baseState.GetTrigger())
	})

	t.Run("Should correctly return input", func(t *testing.T) {
		assert.Equal(t, input, baseState.GetInput())
	})

	t.Run("Should correctly return output", func(t *testing.T) {
		assert.Equal(t, output, baseState.GetOutput())
	})

	t.Run("Should correctly return error", func(t *testing.T) {
		assert.Equal(t, testErr, baseState.GetError())
	})
}

func TestBaseState_Setters(t *testing.T) {
	baseState := &BaseState{}

	t.Run("Should correctly set status", func(t *testing.T) {
		baseState.SetStatus(nats.StatusRunning)
		assert.Equal(t, nats.StatusRunning, baseState.Status)
	})

	t.Run("Should correctly set error", func(t *testing.T) {
		testErr := &Error{Message: "error message", Code: "ERR_CODE"}
		baseState.SetError(testErr)
		assert.Equal(t, testErr, baseState.Error)
	})
}

func TestBaseState_WithEnv(t *testing.T) {
	baseState := &BaseState{
		Env: &common.EnvMap{"EXISTING": "value", "SHARED": "original"},
	}
	newEnv := common.EnvMap{"NEW": "new-value", "SHARED": "updated"}
	err := baseState.WithEnv(newEnv)
	require.NoError(t, err)
	assert.Equal(t, "value", (*baseState.Env)["EXISTING"])
	assert.Equal(t, "new-value", (*baseState.Env)["NEW"])
	assert.Equal(t, "updated", (*baseState.Env)["SHARED"]) // Value should be overridden
}

func TestBaseState_WithInput(t *testing.T) {
	baseState := &BaseState{
		Input: &common.Input{"existing": "value", "shared": "original"},
	}
	newInput := common.Input{"new": "new-value", "shared": "updated"}
	err := baseState.WithInput(newInput)
	require.NoError(t, err)
	assert.Equal(t, "value", (*baseState.Input)["existing"])
	assert.Equal(t, "new-value", (*baseState.Input)["new"])
	assert.Equal(t, "updated", (*baseState.Input)["shared"]) // Value should be overridden
}

func TestStateMap(t *testing.T) {
	corrID, err := common.NewID()
	require.NoError(t, err)
	execID1, err := common.NewID()
	require.NoError(t, err)
	execID2, err := common.NewID()
	require.NoError(t, err)

	id1 := NewID(nats.ComponentWorkflow, corrID, execID1)
	id2 := NewID(nats.ComponentTask, corrID, execID2)
	state1 := NewEmptyState(OptsWithID(id1))
	state2 := NewEmptyState(OptsWithID(id2))
	stateMap := Map{}

	t.Run("Should add states to map", func(t *testing.T) {
		stateMap.Add(state1)
		stateMap.Add(state2)
		assert.Len(t, stateMap, 2)
	})

	t.Run("Should get existing state", func(t *testing.T) {
		gotState1, exists := stateMap.Get(id1)
		assert.True(t, exists)
		assert.Equal(t, state1, gotState1)
		gotState2, exists := stateMap.Get(id2)
		assert.True(t, exists)
		assert.Equal(t, state2, gotState2)
	})

	t.Run("Should return false for non-existent state", func(t *testing.T) {
		nonExistentID := NewID(nats.ComponentAgent, corrID, execID1)
		_, exists := stateMap.Get(nonExistentID)
		assert.False(t, exists)
	})

	t.Run("Should remove state", func(t *testing.T) {
		stateMap.Remove(id1)
		_, exists := stateMap.Get(id1)
		assert.False(t, exists)
		_, exists = stateMap.Get(id2)
		assert.True(t, exists)
	})
}
