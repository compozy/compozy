package test

import (
	"context"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowState(t *testing.T) {
	// Create a base temp dir for all tests
	baseTestDir, err := os.MkdirTemp("", "workflow-state-tests-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseTestDir)

	ctx := context.Background()
	natsServer, natsClient := utils.SetupNatsServer(t, ctx)
	defer natsServer.Shutdown()
	defer natsClient.Close()

	t.Run("State initialization", func(t *testing.T) {
		// Create input for workflow execution
		triggerInput := &common.Input{
			"key1": "value1",
			"key2": 42,
		}

		// Create project environment
		projectEnv := common.EnvMap{
			"PROJECT_ENV": "project_value",
		}

		// Create a new workflow execution
		exec := workflow.NewExecution(triggerInput, projectEnv)
		require.NotNil(t, exec)
		assert.NotEmpty(t, exec.CorrID)
		assert.NotEmpty(t, exec.ExecID)
		assert.Equal(t, triggerInput, exec.TriggerInput)
		assert.Equal(t, projectEnv, exec.ProjectEnv)

		// Add workflow environment
		exec.WorkflowEnv = common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
		}

		// Initialize state
		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		// Verify state properties
		assert.Equal(t, nats.StatusPending, wfState.Status)
		assert.Equal(t, exec.ExecID, wfState.WorkflowExecID)
		assert.NotNil(t, wfState.Env)
		assert.Equal(t, "project_value", (*wfState.Env)["PROJECT_ENV"])
		assert.Equal(t, "workflow_value", (*wfState.Env)["WORKFLOW_ENV"])
		assert.Equal(t, triggerInput, wfState.Trigger)
	})

	t.Run("State persistence", func(t *testing.T) {
		// Create temporary directory for state manager
		tempDir, err := os.MkdirTemp(baseTestDir, "state-persistence-*")
		require.NoError(t, err)

		// Create state manager
		manager, err := state.NewManager(
			state.WithDataDir(tempDir),
			state.WithNatsClient(natsClient),
		)
		require.NoError(t, err)
		defer manager.Close()

		// Create test workflow execution
		triggerInput := &common.Input{
			"request_id": "test-123",
			"payload": map[string]any{
				"message": "Hello, world!",
			},
		}
		projectEnv := common.EnvMap{
			"ENV_VAR1": "value1",
		}
		exec := workflow.NewExecution(triggerInput, projectEnv)

		// Initialize workflow state
		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)

		// Store the state
		err = manager.SaveState(wfState)
		require.NoError(t, err)

		// Retrieve the state
		retrievedState, err := manager.GetWorkflowState(exec.CorrID, exec.ExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedState)

		// Verify retrieved state
		assert.Equal(t, wfState.StateID, retrievedState.GetID())
		assert.Equal(t, nats.StatusPending, retrievedState.GetStatus())
		assert.Equal(t, exec.ExecID, retrievedState.GetID().ExecID)
	})

	t.Run("State updates", func(t *testing.T) {
		// Create temporary directory for state manager
		tempDir, err := os.MkdirTemp(baseTestDir, "state-updates-*")
		require.NoError(t, err)

		// Create state manager
		manager, err := state.NewManager(
			state.WithDataDir(tempDir),
			state.WithNatsClient(natsClient),
		)
		require.NoError(t, err)
		defer manager.Close()

		// Create test workflow execution
		exec := workflow.NewExecution(&common.Input{}, common.EnvMap{})

		// Initialize workflow state
		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)

		// Store the initial state
		err = manager.SaveState(wfState)
		require.NoError(t, err)

		// Update state properties
		wfState.Status = nats.StatusRunning
		wfState.Output = &common.Output{
			"result": "processing",
		}

		// Update the state
		err = manager.SaveState(wfState)
		require.NoError(t, err)

		// Retrieve the updated state
		retrievedState, err := manager.GetWorkflowState(exec.CorrID, exec.ExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedState)

		// Verify updated state
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
		assert.Equal(t, "processing", (*retrievedState.GetOutput())["result"])
	})

	t.Run("Environment merging and normalization", func(t *testing.T) {
		// Create input with values to be referenced in templates
		triggerInput := &common.Input{
			"username":   "testuser",
			"request_id": "req-123",
			"data": map[string]any{
				"action":   "process",
				"priority": "high",
			},
		}

		// Create project environment with a template reference
		projectEnv := common.EnvMap{
			"PROJECT_ENV": "project_value",
			"USER":        "{{ trigger.username }}",
		}

		// Create workflow environment with both normal values and templates
		workflowEnv := common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
			"REQUEST_ID":   "{{ trigger.request_id }}",
			"ACTION":       "{{ trigger.data.action }}",
			"PROJECT_ENV":  "workflow_override", // Should override project env
		}

		// Create a new workflow execution
		exec := workflow.NewExecution(triggerInput, projectEnv)
		exec.WorkflowEnv = workflowEnv

		// Initialize state
		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		// Verify environment merging (workflow env should override project env)
		assert.Equal(t, "workflow_override", (*wfState.Env)["PROJECT_ENV"])
		assert.Equal(t, "workflow_value", (*wfState.Env)["WORKFLOW_ENV"])

		// Verify template normalization in environment variables
		assert.Equal(t, "testuser", (*wfState.Env)["USER"])
		assert.Equal(t, "req-123", (*wfState.Env)["REQUEST_ID"])
		assert.Equal(t, "process", (*wfState.Env)["ACTION"])
	})
}
