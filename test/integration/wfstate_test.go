package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowStateInitialization(t *testing.T) {
	// Setup common to initialization tests
	triggerInput := &common.Input{
		"key1": "value1",
		"key2": 42,
	}
	projectEnv := common.EnvMap{
		"PROJECT_ENV": "project_value",
	}
	exec := workflow.NewExecution(triggerInput, projectEnv)
	require.NotNil(t, exec)

	t.Run("Should correctly initialize IDs, status, and basic fields", func(t *testing.T) {
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

	t.Run("Should correctly merge and normalize environment variables", func(t *testing.T) {
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
			"USER":        "{{ .trigger.input.username }}",
		}
		workflowEnv := common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
			"REQUEST_ID":   "{{ .trigger.input.request_id }}",
			"ACTION":       "{{ .trigger.input.data.action }}",
			"PROJECT_ENV":  "workflow_override", // Workflow env should override project env
		}
		exec := workflow.NewExecution(triggerInput, projectEnv)
		exec.WorkflowEnv = workflowEnv

		wfState, err := workflow.NewState(exec)
		require.NoError(t, err)
		require.NotNil(t, wfState)
		require.NotNil(t, wfState.Env)

		assert.Equal(t, "workflow_override", (*wfState.Env)["PROJECT_ENV"])
		assert.Equal(t, "workflow_value", (*wfState.Env)["WORKFLOW_ENV"])
		assert.Equal(t, "testuser", (*wfState.Env)["USER"])
		assert.Equal(t, "req-123", (*wfState.Env)["REQUEST_ID"])
		assert.Equal(t, "process", (*wfState.Env)["ACTION"])
	})
}

func TestWorkflowStatePersistence(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

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
	require.NotNil(t, exec)

	wfState, err := workflow.NewState(exec)
	require.NoError(t, err)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	retrievedStateInterface, err := stateManager.GetWorkflowState(exec.CorrID, exec.ExecID)
	require.NoError(t, err)
	require.NotNil(t, retrievedStateInterface)

	retrievedBaseState, ok := retrievedStateInterface.(*state.BaseState)
	require.True(t, ok, "Retrieved state should be *state.BaseState")

	assert.Equal(t, wfState.GetID(), retrievedBaseState.GetID())
	assert.Equal(t, nats.ComponentWorkflow, retrievedBaseState.GetID().Component)
	assert.Equal(t, nats.StatusPending, retrievedBaseState.GetStatus())
	assert.Equal(t, exec.ExecID, retrievedBaseState.GetID().ExecID)
	assert.Equal(t, *wfState.GetEnv(), *retrievedBaseState.GetEnv())
	assert.Equal(t, *wfState.GetTrigger(), *retrievedBaseState.GetTrigger())
	assert.Equal(t, *wfState.GetOutput(), *retrievedBaseState.GetOutput())
}

func TestWorkflowStateUpdates(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	exec := workflow.NewExecution(&common.Input{}, common.EnvMap{})
	require.NotNil(t, exec)

	wfState, err := workflow.NewState(exec)
	require.NoError(t, err)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	// Update status and output
	wfState.SetStatus(nats.StatusRunning)
	updatedOutput := &common.Output{
		"result": "processing",
	}
	wfState.Output = updatedOutput // Direct assignment for BaseState.Output

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	retrievedStateInterface, err := stateManager.GetWorkflowState(exec.CorrID, exec.ExecID)
	require.NoError(t, err)
	require.NotNil(t, retrievedStateInterface)

	retrievedBaseState, ok := retrievedStateInterface.(*state.BaseState)
	require.True(t, ok, "Retrieved state should be *state.BaseState")

	assert.Equal(t, nats.StatusRunning, retrievedBaseState.GetStatus())
	require.NotNil(t, retrievedBaseState.GetOutput())
	assert.Equal(t, *updatedOutput, *retrievedBaseState.GetOutput())
	assert.Equal(t, *wfState.GetEnv(), *retrievedBaseState.GetEnv()) // Ensure other fields remain consistent
}
