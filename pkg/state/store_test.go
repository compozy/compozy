package state

import (
	"os"
	"testing"

	"github.com/compozy/compozy/engine/common"
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
		assert.Equal(t, DefaultStorePrefixes, store.prefixes)

		// Clean up
		err = store.Close()
		require.NoError(t, err)
	})

	t.Run("NewStore with options", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "store-test-options-*")
		require.NoError(t, err)

		// Custom prefixes for testing
		customPrefixes := StorePrefixes{
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
		id := NewID(nats.ComponentWorkflow, "workflow-1", "correlation-1")
		testState := &BaseState{
			ID:      id,
			Status:  nats.StatusPending,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
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
		id := NewID(nats.ComponentWorkflow, "workflow-1", "correlation-1")
		testState := &BaseState{
			ID:      id,
			Status:  nats.StatusPending,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}

		// Test UpsertState
		err = store.UpsertState(testState)
		require.NoError(t, err)

		// Update the state
		testState.Status = nats.StatusRunning
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
		id := NewID(nats.ComponentWorkflow, "workflow-1", "correlation-1")
		testState := &BaseState{
			ID:      id,
			Status:  nats.StatusPending,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}

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
		correlationID := "correlation-1"

		// Create workflow state
		workflowID := NewID(nats.ComponentWorkflow, "workflow-1", correlationID)
		workflowState := &BaseState{
			ID:      workflowID,
			Status:  nats.StatusRunning,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = store.UpsertState(workflowState)
		require.NoError(t, err)

		// Create task states
		taskID1 := NewID(nats.ComponentTask, "task-1", correlationID)
		taskState1 := &BaseState{
			ID:      taskID1,
			Status:  nats.StatusSuccess,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = store.UpsertState(taskState1)
		require.NoError(t, err)

		taskID2 := NewID(nats.ComponentTask, "task-2", correlationID)
		taskState2 := &BaseState{
			ID:      taskID2,
			Status:  nats.StatusRunning,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = store.UpsertState(taskState2)
		require.NoError(t, err)

		// Create agent state
		agentID := NewID(nats.ComponentAgent, "agent-1", correlationID)
		agentState := &BaseState{
			ID:      agentID,
			Status:  nats.StatusRunning,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = store.UpsertState(agentState)
		require.NoError(t, err)

		// Create tool state
		toolID := NewID(nats.ComponentTool, "tool-1", correlationID)
		toolState := &BaseState{
			ID:      toolID,
			Status:  nats.StatusSuccess,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = store.UpsertState(toolState)
		require.NoError(t, err)

		// Test GetTaskStatesForWorkflow
		taskStates, err := store.GetTaskStatesForWorkflow(workflowID)
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
