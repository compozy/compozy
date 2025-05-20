package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/common"
	natspkg "github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	// Create a base temp dir for all tests
	baseTestDir, err := os.MkdirTemp("", "state-manager-tests-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseTestDir)

	// Start an embedded NATS server
	opts := natspkg.DefaultServerOptions()
	opts.EnableJetStream = true

	natsServer, err := natspkg.NewNatsServer(opts)
	require.NoError(t, err, "Failed to start NATS server")
	defer natsServer.Shutdown()

	// Create a client for tests
	natsClient := natspkg.NewClient(natsServer.Conn)
	defer natsClient.Close()

	t.Run("NewManager", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-*")
		require.NoError(t, err)

		// Create a test config
		config := ManagerConfig{
			DataDir:    tempDir,
			NatsClient: natsClient,
			StreamsToHandle: []string{
				string(natspkg.ComponentWorkflow),
				string(natspkg.ComponentTask),
				string(natspkg.ComponentAgent),
				string(natspkg.ComponentTool),
			},
		}

		// Test manager creation
		manager, err := NewManager(context.Background(), config)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Clean up
		err = manager.Close()
		require.NoError(t, err)
	})

	t.Run("NewManager with nil client", func(t *testing.T) {
		// Test with nil NATS client
		config := ManagerConfig{
			DataDir:    filepath.Join(baseTestDir, "nil-client-test"),
			NatsClient: nil,
		}

		// Expect error when NATS client is nil
		_, err := NewManager(context.Background(), config)
		require.Error(t, err)
	})

	t.Run("Default configuration", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-defaults-*")
		require.NoError(t, err)

		// Create a test config with minimal fields but explicit data dir
		config := ManagerConfig{
			NatsClient: natsClient,
			DataDir:    tempDir, // Override default to use temp dir
		}

		// Expect defaults to be used for other fields
		manager, err := NewManager(context.Background(), config)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Verify that streams use default but dataDir is our temp dir
		defaultConfig := DefaultManagerConfig()
		assert.Equal(t, tempDir, manager.dataDir) // Should use our temp dir, not default
		assert.Equal(t, len(defaultConfig.StreamsToHandle), len(manager.streams))

		// Clean up
		err = manager.Close()
		require.NoError(t, err)
	})

	t.Run("State retrieval methods", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp(baseTestDir, "manager-test-retrieval-*")
		require.NoError(t, err)

		// Create a test config
		config := ManagerConfig{
			DataDir:    tempDir,
			NatsClient: natsClient,
		}

		// Create manager
		manager, err := NewManager(context.Background(), config)
		require.NoError(t, err)
		defer manager.Close()

		// Common test data
		var wfID string
		wfID = "workflow-1"
		corrID := "correlation-1"

		// Add a workflow state directly to the store
		wfStateID := NewID(natspkg.ComponentWorkflow, wfID, corrID)
		wfState := &BaseState{
			ID:      wfStateID,
			Status:  natspkg.StatusRunning,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = manager.store.UpsertState(wfState)
		require.NoError(t, err)

		// Add a task state
		tID := "task-1"
		tStateID := NewID(natspkg.ComponentTask, tID, corrID)
		taskState := &BaseState{
			ID:      tStateID,
			Status:  natspkg.StatusPending,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = manager.store.UpsertState(taskState)
		require.NoError(t, err)

		// Add an agent state
		agID := "agent-1"
		aStateID := NewID(natspkg.ComponentAgent, agID, corrID)
		agState := &BaseState{
			ID:      aStateID,
			Status:  natspkg.StatusRunning,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = manager.store.UpsertState(agState)
		require.NoError(t, err)

		// Add a tool state
		toolID := "tool-1"
		toolStateID := NewID(natspkg.ComponentTool, toolID, corrID)
		toolState := &BaseState{
			ID:      toolStateID,
			Status:  natspkg.StatusSuccess,
			Input:   make(common.Input),
			Output:  make(common.Output),
			Env:     make(common.EnvMap),
			Trigger: make(common.Input),
		}
		err = manager.store.UpsertState(toolState)
		require.NoError(t, err)

		// Test GetWorkflowState
		state, err := manager.GetWorkflowState(wfID, corrID)
		require.NoError(t, err)
		assert.Equal(t, wfStateID, state.GetID())
		assert.Equal(t, natspkg.StatusRunning, state.GetStatus())

		// Test GetTaskState
		state, err = manager.GetTaskState(tID, corrID)
		require.NoError(t, err)
		assert.Equal(t, tStateID, state.GetID())
		assert.Equal(t, natspkg.StatusPending, state.GetStatus())

		// Test GetAgentState
		state, err = manager.GetAgentState(agID, corrID)
		require.NoError(t, err)
		assert.Equal(t, aStateID, state.GetID())
		assert.Equal(t, natspkg.StatusRunning, state.GetStatus())

		// Test GetToolState
		state, err = manager.GetToolState(toolID, corrID)
		require.NoError(t, err)
		assert.Equal(t, toolStateID, state.GetID())
		assert.Equal(t, natspkg.StatusSuccess, state.GetStatus())

		// Test GetTaskStatesForWorkflow
		taskStates, err := manager.GetTaskStatesForWorkflow(wfID, corrID)
		require.NoError(t, err)
		assert.Len(t, taskStates, 1)

		// Test GetAgentStatesForTask
		agentStates, err := manager.GetAgentStatesForTask(tID, corrID)
		require.NoError(t, err)
		assert.Len(t, agentStates, 1)

		// Test GetToolStatesForTask
		toolStates, err := manager.GetToolStatesForTask(tID, corrID)
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
		err = manager.DeleteWorkflowState(wfID, corrID)
		require.NoError(t, err)

		// Verify all related states are deleted
		_, err = manager.GetWorkflowState(wfID, corrID)
		require.Error(t, err)
		_, err = manager.GetTaskState(tID, corrID)
		require.Error(t, err)
		_, err = manager.GetAgentState(agID, corrID)
		require.Error(t, err)
		_, err = manager.GetToolState(toolID, corrID)
		require.Error(t, err)
	})
}
