package core

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Run("SetMain should set the main state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		mainState := &MockState{id: stateID}

		var state State = mainState
		gc.SetWorkflow(state)

		assert.Equal(t, state, *gc.Workflow, "Main state should be set correctly")
	})

	t.Run("UpsertTask should add new task state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		taskState := &MockState{id: stateID}

		var state State = taskState
		err := gc.UpsertTask(&state)
		require.NoError(t, err)

		retrieved, exists := gc.GetTask(*stateID)
		assert.True(t, exists, "Task should exist in map")
		assert.Equal(t, taskState, retrieved, "Retrieved task should match added task")
	})

	t.Run("UpsertTask should update existing task state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		taskState1 := &MockState{id: stateID}
		taskState2 := &MockState{id: stateID}

		var state1 State = taskState1
		var state2 State = taskState2

		err := gc.UpsertTask(&state1)
		require.NoError(t, err)
		err = gc.UpsertTask(&state2)
		require.NoError(t, err)

		retrieved, exists := gc.GetTask(*stateID)
		assert.True(t, exists, "Task should exist in map")
		assert.Equal(t, taskState2, retrieved, "Retrieved task should match updated task")
	})

	t.Run("UpsertTask should handle nil state", func(t *testing.T) {
		gc := &Store{}

		err := gc.UpsertTask(nil)
		assert.Error(t, err, "Should error on nil state")
	})

	t.Run("UpsertTool should add new tool state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		toolState := &MockState{id: stateID}

		var state State = toolState
		err := gc.UpsertTool(&state)
		require.NoError(t, err)

		retrieved, exists := gc.GetTool(*stateID)
		assert.True(t, exists, "Tool should exist in map")
		assert.Equal(t, toolState, retrieved, "Retrieved tool should match added tool")
	})

	t.Run("UpsertTool should update existing tool state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		toolState1 := &MockState{id: stateID}
		toolState2 := &MockState{id: stateID}

		var state1 State = toolState1
		var state2 State = toolState2

		err := gc.UpsertTool(&state1)
		require.NoError(t, err)
		err = gc.UpsertTool(&state2)
		require.NoError(t, err)

		retrieved, exists := gc.GetTool(*stateID)
		assert.True(t, exists, "Tool should exist in map")
		assert.Equal(t, toolState2, retrieved, "Retrieved tool should match updated tool")
	})

	t.Run("UpsertTool should handle nil state", func(t *testing.T) {
		gc := &Store{}

		err := gc.UpsertTool(nil)
		assert.Error(t, err, "Should error on nil state")
	})

	t.Run("UpsertAgent should add new agent state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		agentState := &MockState{id: stateID}

		var state State = agentState
		err := gc.UpsertAgent(&state)
		require.NoError(t, err)

		retrieved, exists := gc.GetAgent(*stateID)
		assert.True(t, exists, "Agent should exist in map")
		assert.Equal(t, agentState, retrieved, "Retrieved agent should match added agent")
	})

	t.Run("UpsertAgent should update existing agent state", func(t *testing.T) {
		gc := &Store{}
		_, stateID := createIDs()
		agentState1 := &MockState{id: stateID}
		agentState2 := &MockState{id: stateID}

		var state1 State = agentState1
		var state2 State = agentState2

		err := gc.UpsertAgent(&state1)
		require.NoError(t, err)
		err = gc.UpsertAgent(&state2)
		require.NoError(t, err)

		retrieved, exists := gc.GetAgent(*stateID)
		assert.True(t, exists, "Agent should exist in map")
		assert.Equal(t, agentState2, retrieved, "Retrieved agent should match updated agent")
	})

	t.Run("UpsertAgent should handle nil state", func(t *testing.T) {
		gc := &Store{}

		err := gc.UpsertAgent(nil)
		assert.Error(t, err, "Should error on nil state")
	})

	invalidID := StateID{
		Component:   common.ComponentWorkflow,
		ComponentID: "non-existent",
		ExecID:      "non-existent",
	}

	t.Run("GetTask should return false for non-existent task", func(t *testing.T) {
		gc := &Store{}

		_, exists := gc.GetTask(invalidID)
		assert.False(t, exists, "Non-existent task should not be found")
	})

	t.Run("GetTool should return false for non-existent tool", func(t *testing.T) {
		gc := &Store{}

		_, exists := gc.GetTool(invalidID)
		assert.False(t, exists, "Non-existent tool should not be found")
	})

	t.Run("GetAgent should return false for non-existent agent", func(t *testing.T) {
		gc := &Store{}

		_, exists := gc.GetAgent(invalidID)
		assert.False(t, exists, "Non-existent agent should not be found")
	})
}
