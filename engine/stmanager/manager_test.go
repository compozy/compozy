package stmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	// Create a base temp dir for all tests
	baseTestDir, err := os.MkdirTemp("", "state-manager-tests-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseTestDir)

	ctx := context.Background()
	natsServer, natsClient := utils.SetupNatsServer(ctx, t)
	defer natsServer.Shutdown()
	defer natsClient.Close()

	t.Run("NewManager", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-*")
		require.NoError(t, err)

		store, err := store.NewStore(tempDir)
		require.NoError(t, err)

		manager, err := NewManager(
			WithStore(store),
			WithNatsClient(natsClient),
			WithComponents(defaultConsumers()),
		)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Clean up
		err = manager.Close()
		require.NoError(t, err)
	})

	t.Run("NewManager with nil client", func(t *testing.T) {
		// Test with nil NATS client
		dataDir := filepath.Join(baseTestDir, "nil-client-test")
		store, err := store.NewStore(dataDir)
		require.NoError(t, err)
		_, err = NewManager(
			WithStore(store),
		)
		require.Error(t, err)
	})

	t.Run("Default configuration", func(t *testing.T) {
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-defaults-*")
		require.NoError(t, err)

		store, err := store.NewStore(tempDir)
		require.NoError(t, err)

		manager, err := NewManager(
			WithStore(store),
			WithNatsClient(natsClient),
		)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Verify that streams use default but dataDir is our temp dir
		defaultStreamsLength := len(defaultConsumers())
		assert.Equal(t, tempDir, store.DataDir())
		assert.Equal(t, defaultStreamsLength, len(manager.components))

		// Clean up
		err = manager.Close()
		require.NoError(t, err)
	})

	t.Run("State retrieval methods", func(t *testing.T) {
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-retrieval-*")
		require.NoError(t, err)

		store, err := store.NewStore(tempDir)
		require.NoError(t, err)

		manager, err := NewManager(
			WithStore(store),
			WithNatsClient(natsClient),
		)
		require.NoError(t, err)
		defer manager.Close()

		corrID, err := common.NewID()
		require.NoError(t, err)
		workflowID, err := common.NewID()
		require.NoError(t, err)
		wfStateID := state.NewID(nats.ComponentWorkflow, corrID, workflowID)
		wfState := &state.BaseState{
			StateID: wfStateID,
			Status:  nats.StatusRunning,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}
		err = manager.store.UpsertState(wfState)
		require.NoError(t, err)

		// Add a task state
		taskID, err := common.NewID()
		require.NoError(t, err)
		tStateID := state.NewID(nats.ComponentTask, corrID, taskID)
		taskState := &state.BaseState{
			StateID: tStateID,
			Status:  nats.StatusPending,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}
		err = manager.store.UpsertState(taskState)
		require.NoError(t, err)

		// Add an agent state
		agentID, err := common.NewID()
		require.NoError(t, err)
		aStateID := state.NewID(nats.ComponentAgent, corrID, agentID)
		agState := &state.BaseState{
			StateID: aStateID,
			Status:  nats.StatusRunning,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}
		err = manager.store.UpsertState(agState)
		require.NoError(t, err)

		// Add a tool state
		toolID, err := common.NewID()
		require.NoError(t, err)
		toolStateID := state.NewID(nats.ComponentTool, corrID, toolID)
		toolState := &state.BaseState{
			StateID: toolStateID,
			Status:  nats.StatusSuccess,
			Input:   &common.Input{},
			Output:  &common.Output{},
			Env:     &common.EnvMap{},
			Trigger: &common.Input{},
		}
		err = manager.store.UpsertState(toolState)
		require.NoError(t, err)

		// Test GetWorkflowState
		state, err := manager.GetWorkflowState(corrID, workflowID)
		require.NoError(t, err)
		assert.Equal(t, wfStateID, state.GetID())
		assert.Equal(t, nats.StatusRunning, state.GetStatus())

		// Test GetTaskState
		state, err = manager.GetTaskState(corrID, taskID)
		require.NoError(t, err)
		assert.Equal(t, tStateID, state.GetID())
		assert.Equal(t, nats.StatusPending, state.GetStatus())

		// Test GetAgentState
		state, err = manager.GetAgentState(corrID, agentID)
		require.NoError(t, err)
		assert.Equal(t, aStateID, state.GetID())
		assert.Equal(t, nats.StatusRunning, state.GetStatus())

		// Test GetToolState
		state, err = manager.GetToolState(corrID, toolID)
		require.NoError(t, err)
		assert.Equal(t, toolStateID, state.GetID())
		assert.Equal(t, nats.StatusSuccess, state.GetStatus())

		// Test GetTaskStatesForWorkflow
		taskStates, err := manager.GetTaskStatesForWorkflow(corrID, workflowID)
		require.NoError(t, err)
		assert.Len(t, taskStates, 1)

		// Test GetAgentStatesForTask
		agentStates, err := manager.GetAgentStatesForTask(corrID, taskID)
		require.NoError(t, err)
		assert.Len(t, agentStates, 1)

		// Test GetToolStatesForTask
		toolStates, err := manager.GetToolStatesForTask(corrID, taskID)
		require.NoError(t, err)
		assert.Len(t, toolStates, 1)

		// Test GetAllWorkflowStates
		workflowStates, err := manager.GetAllWorkflowStates()
		require.NoError(t, err)
		assert.Len(t, workflowStates, 1)

		// Test GetAllTaskStates
		allTaskStates, err := manager.GetAllTaskStates()
		require.NoError(t, err)
		assert.Len(t, allTaskStates, 1)

		// Test GetAllAgentStates
		allAgentStates, err := manager.GetAllAgentStates()
		require.NoError(t, err)
		assert.Len(t, allAgentStates, 1)

		// Test GetAllToolStates
		allToolStates, err := manager.GetAllToolStates()
		require.NoError(t, err)
		assert.Len(t, allToolStates, 1)

		// Test DeleteWorkflowState
		err = manager.DeleteWorkflowState(corrID, workflowID)
		require.NoError(t, err)

		// Verify all related states are deleted
		_, err = manager.GetWorkflowState(corrID, workflowID)
		require.Error(t, err)
		_, err = manager.GetTaskState(corrID, taskID)
		require.Error(t, err)
		_, err = manager.GetAgentState(corrID, agentID)
		require.Error(t, err)
		_, err = manager.GetToolState(corrID, toolID)
		require.Error(t, err)
	})
}
