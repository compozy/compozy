package store

import (
	"os"
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	// Create a base temp dir for all tests
	baseTestDir, err := os.MkdirTemp("", "state-store-tests-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseTestDir)

	t.Run("NewStore", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-*")
		require.NoError(t, err)

		// Test store creation
		store, err := NewStore(tempDir)
		require.NoError(t, err)
		require.NotNil(t, store)

		// Verify store has default prefixes
		assert.Equal(t, DefaultPrefixes, store.prefixes)

		// Clean up
		err = store.Close()
		require.NoError(t, err)
	})

	t.Run("NewStore with options", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-options-*")
		require.NoError(t, err)

		// Custom prefixes for testing
		customPrefixes := Prefixes{
			Workflow: "w:",
			Task:     "t:",
			Agent:    "a:",
			Tool:     "tl:",
		}

		// Test store creation with options
		store, err := NewStore(tempDir, WithPrefixes(customPrefixes))
		require.NoError(t, err)
		require.NotNil(t, store)

		// Verify store has custom prefixes
		assert.Equal(t, customPrefixes, store.prefixes)

		// Clean up
		err = store.Close()
		require.NoError(t, err)
	})

	t.Run("UpsertState and GetState", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-crud-*")
		require.NoError(t, err)

		// Create store
		store, err := NewStore(tempDir)
		require.NoError(t, err)
		defer store.Close()

		// Create a test state
		id := state.NewID(nats.ComponentWorkflow, "correlation-1", "exec-1")
		testState := &state.BaseState{
			StateID: id,
			Status:  nats.StatusPending,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}

		// Test UpsertState
		err = store.UpsertState(testState)
		require.NoError(t, err)

		// Test GetState
		retrievedState, err := store.GetState(id)
		require.NoError(t, err)
		require.NotNil(t, retrievedState)

		// Verify the retrieved state
		assert.Equal(t, id, retrievedState.GetID())
		assert.Equal(t, nats.StatusPending, retrievedState.GetStatus())
	})

	t.Run("UpsertState, update, and GetState", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-update-*")
		require.NoError(t, err)

		// Create store
		store, err := NewStore(tempDir)
		require.NoError(t, err)
		defer store.Close()

		// Create a test state
		id := state.NewID(nats.ComponentWorkflow, "correlation-1", "exec-1")
		testState := state.NewEmptyState(state.WithID(id))
		// Test UpsertState
		err = store.UpsertState(testState)
		require.NoError(t, err)

		// Update the state
		testState.SetStatus(nats.StatusRunning)
		err = store.UpsertState(testState)
		require.NoError(t, err)

		// Test GetState
		retrievedState, err := store.GetState(id)
		require.NoError(t, err)
		require.NotNil(t, retrievedState)

		// Verify the retrieved state has the updated status
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("DeleteState", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-delete-*")
		require.NoError(t, err)

		// Create store
		store, err := NewStore(tempDir)
		require.NoError(t, err)
		defer store.Close()

		// Create a test state
		id := state.NewID(nats.ComponentWorkflow, "correlation-1", "exec-1")
		testState := state.NewEmptyState(state.WithID(id))

		// Add the state
		err = store.UpsertState(testState)
		require.NoError(t, err)

		// Verify the state exists
		_, err = store.GetState(id)
		require.NoError(t, err)

		// Delete the state
		err = store.DeleteState(id)
		require.NoError(t, err)

		// Verify the state doesn't exist anymore
		_, err = store.GetState(id)
		require.Error(t, err)
	})

	t.Run("Query operations", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-query-*")
		require.NoError(t, err)

		// Create store
		store, err := NewStore(tempDir)
		require.NoError(t, err)
		defer store.Close()

		// Common correlation ID
		corrID := common.NewCorrID()

		// Create workflow state
		wfID := state.NewID(nats.ComponentWorkflow, corrID, "workflow-1")
		wfState := &state.BaseState{
			StateID: wfID,
			Status:  nats.StatusRunning,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}
		err = store.UpsertState(wfState)
		require.NoError(t, err)

		// Create task states
		taskID1 := state.NewID(nats.ComponentTask, corrID, "task-1")
		taskState1 := state.NewEmptyState(
			state.WithID(taskID1),
			state.WithStatus(nats.StatusSuccess),
		)
		err = store.UpsertState(taskState1)
		require.NoError(t, err)

		taskID2 := state.NewID(nats.ComponentTask, corrID, "task-2")
		taskState2 := state.NewEmptyState(
			state.WithID(taskID2),
			state.WithStatus(nats.StatusRunning),
		)
		err = store.UpsertState(taskState2)
		require.NoError(t, err)

		// Create agent state
		agID := state.NewID(nats.ComponentAgent, corrID, "agent-1")
		agState := state.NewEmptyState(
			state.WithID(agID),
			state.WithStatus(nats.StatusRunning),
		)
		err = store.UpsertState(agState)
		require.NoError(t, err)

		// Create tool state
		toolID := state.NewID(nats.ComponentTool, corrID, "tool-1")
		toolState := state.NewEmptyState(
			state.WithID(toolID),
			state.WithStatus(nats.StatusSuccess),
		)
		err = store.UpsertState(toolState)
		require.NoError(t, err)

		// Test GetTaskStatesForWorkflow
		taskStates, err := store.GetTaskStatesForWorkflow(wfID)
		require.NoError(t, err)
		assert.Len(t, taskStates, 2)

		// Test GetAgentStatesForTask
		agentStates, err := store.GetAgentStatesForTask(taskID1)
		require.NoError(t, err)
		assert.Len(t, agentStates, 1)

		// Test GetToolStatesForTask
		toolStates, err := store.GetToolStatesForTask(taskID1)
		require.NoError(t, err)
		assert.Len(t, toolStates, 1)

		// Test GetStatesByComponent
		workflowStates, err := store.GetStatesByComponent(nats.ComponentWorkflow)
		require.NoError(t, err)
		assert.Len(t, workflowStates, 1)

		taskStates, err = store.GetStatesByComponent(nats.ComponentTask)
		require.NoError(t, err)
		assert.Len(t, taskStates, 2)

		agentStates, err = store.GetStatesByComponent(nats.ComponentAgent)
		require.NoError(t, err)
		assert.Len(t, agentStates, 1)

		toolStates, err = store.GetStatesByComponent(nats.ComponentTool)
		require.NoError(t, err)
		assert.Len(t, toolStates, 1)
	})
}
