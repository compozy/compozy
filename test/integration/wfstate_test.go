package test

import (
	"context"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state" // For state.BaseState
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: TestMain is now expected to be in test_helpers.go to manage GlobalBaseTestDir

func TestWorkflowState(t *testing.T) {
	// Create a base temp dir for this specific test function's lifecycle.
	// This is different from GlobalBaseTestDir used by IntegrationTestBed.
	// If wfstate_test.go were to fully use IntegrationTestBed for each sub-test as a top-level test,
	// this would be handled by GlobalBaseTestDir.
	workflowFuncBaseTestDir, err := os.MkdirTemp("", "workflow-state-func-tests-*")
	require.NoError(t, err)
	defer os.RemoveAll(workflowFuncBaseTestDir)

	ctx := context.Background() // A single context for NATS server for all sub-tests
	natsServer, natsClient := utils.SetupNatsServer(t, ctx)
	defer natsServer.Shutdown()
	defer natsClient.Close() // This client is shared by sub-tests for their state managers

	t.Run("State initialization", func(t *testing.T) {
		// This sub-test does not use a state manager, so no changes needed for state manager setup.
		triggerInput := &common.Input{
			"key1": "value1",
			"key2": 42,
		}
		projectEnv := common.EnvMap{
			"PROJECT_ENV": "project_value",
		}
		exec := workflow.NewExecution(triggerInput, projectEnv)
		require.NotNil(t, exec)
		assert.NotEmpty(t, exec.CorrID)
		assert.NotEmpty(t, exec.ExecID)
		assert.Equal(t, triggerInput, exec.TriggerInput)
		assert.Equal(t, projectEnv, exec.ProjectEnv)

		exec.WorkflowEnv = common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
		}

		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		assert.Equal(t, nats.StatusPending, wfState.Status)
		assert.Equal(t, exec.ExecID, wfState.WorkflowExecID)
		assert.NotNil(t, wfState.Env)
		assert.Equal(t, "project_value", (*wfState.Env)["PROJECT_ENV"])
		assert.Equal(t, "workflow_value", (*wfState.Env)["WORKFLOW_ENV"])
		assert.Equal(t, triggerInput, wfState.Trigger)
	})

	t.Run("State persistence", func(t *testing.T) {
		// Use the helper to set up state manager for this sub-test
		// Components for workflow state manager usually include all, or at least Workflow.
		// Using nil for componentsToWatch will use default components in state.Manager.
		manager := SetupStateManagerForSubtest(t, workflowFuncBaseTestDir, natsClient, nil)
		defer manager.Close() // Ensure manager is closed after sub-test

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

		wfState, err := workflow.NewState(exec) // This is *workflow.State
		require.NoError(t, err)

		err = manager.SaveState(wfState)
		require.NoError(t, err)

		retrievedStateInterface, err := manager.GetWorkflowState(exec.CorrID, exec.ExecID) // Returns state.State
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)
		
		retrievedBaseState, ok := retrievedStateInterface.(*state.BaseState)
		require.True(t, ok, "Retrieved state should be *state.BaseState")

		assert.Equal(t, wfState.GetID(), retrievedBaseState.GetID())
		assert.Equal(t, nats.StatusPending, retrievedBaseState.GetStatus())
		assert.Equal(t, exec.ExecID, retrievedBaseState.GetID().ExecID) // Workflow's ExecID is in the StateID
	})

	t.Run("State updates", func(t *testing.T) {
		manager := SetupStateManagerForSubtest(t, workflowFuncBaseTestDir, natsClient, nil)
		defer manager.Close()

		exec := workflow.NewExecution(&common.Input{}, common.EnvMap{})
		wfState, err := workflow.NewState(exec) // *workflow.State
		require.NoError(t, err)

		err = manager.SaveState(wfState)
		require.NoError(t, err)

		// Modify wfState (*workflow.State, which embeds state.BaseState)
		wfState.Status = nats.StatusRunning // Direct field access if Status is public in BaseState
		wfState.Output = &common.Output{    // Direct field access if Output is public in BaseState
			"result": "processing",
		}
		// If Status/Output are not public, use SetStatus() and update BaseState.Output directly
		// wfState.SetStatus(nats.StatusRunning)
		// wfState.BaseState.Output = &common.Output{"result": "processing"}


		err = manager.SaveState(wfState)
		require.NoError(t, err)

		retrievedStateInterface, err := manager.GetWorkflowState(exec.CorrID, exec.ExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)

		retrievedBaseState, ok := retrievedStateInterface.(*state.BaseState)
		require.True(t, ok)
		
		assert.Equal(t, nats.StatusRunning, retrievedBaseState.GetStatus())
		require.NotNil(t, retrievedBaseState.GetOutput())
		assert.Equal(t, "processing", (*retrievedBaseState.GetOutput())["result"])
	})

	t.Run("Environment merging and normalization", func(t *testing.T) {
		// This sub-test does not use a state manager.
		triggerInput := &common.Input{
			"username":   "testuser",
			"request_id": "req-123",
			"data": map[string]any{
				"action":   "process",
				"priority": "high",
			},
		}
		projectEnv := common.EnvMap{
			"PROJECT_ENV": "project_value",
			"USER":        "{{ .trigger.input.username }}", // Corrected template based on normalizer
		}
		workflowEnv := common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
			"REQUEST_ID":   "{{ .trigger.input.request_id }}", // Corrected template
			"ACTION":       "{{ .trigger.input.data.action }}", // Corrected template
			"PROJECT_ENV":  "workflow_override", 
		}
		exec := workflow.NewExecution(triggerInput, projectEnv)
		exec.WorkflowEnv = workflowEnv

		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		assert.Equal(t, "workflow_override", (*wfState.Env)["PROJECT_ENV"])
		assert.Equal(t, "workflow_value", (*wfState.Env)["WORKFLOW_ENV"])
		assert.Equal(t, "testuser", (*wfState.Env)["USER"])
		assert.Equal(t, "req-123", (*wfState.Env)["REQUEST_ID"])
		assert.Equal(t, "process", (*wfState.Env)["ACTION"])
	})
}
