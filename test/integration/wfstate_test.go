package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowStateInitialization(t *testing.T) {
	triggerInput := &common.Input{
		"key1": "value1",
		"key2": 42,
	}
	projectEnv := common.EnvMap{
		"PROJECT_ENV": "project_value",
	}
	wfenv := common.EnvMap{
		"WORKFLOW_ENV": "workflow_value",
	}

	wfInfo := workflow.RandomMetadata("workflow-1")
	stCtx, err := workflow.NewContext(
		wfInfo,
		triggerInput,
		projectEnv,
		wfenv,
	)
	require.NoError(t, err)
	require.NotNil(t, stCtx)

	t.Run("Should correctly initialize IDs, status, and basic fields", func(t *testing.T) {
		assert.NotEmpty(t, stCtx.GetCorrID())
		assert.NotEmpty(t, stCtx.GetWorkflowExecID())
		assert.Equal(t, triggerInput, stCtx.TriggerInput)
		assert.Equal(t, projectEnv, stCtx.ProjectEnv)

		stCtx.WorkflowEnv = common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
		}

		wfState, err := workflow.NewState(stCtx)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		assert.Equal(t, nats.StatusPending, wfState.Status)
		assert.Equal(t, stCtx.GetWorkflowExecID(), wfState.GetContext().GetWorkflowExecID())
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

		wfInfo := workflow.RandomMetadata("workflow-1")
		stCtx, err := workflow.NewContext(
			wfInfo,
			triggerInput,
			projectEnv,
			workflowEnv,
		)
		require.NoError(t, err)
		require.NotNil(t, stCtx)
		stCtx.WorkflowEnv = workflowEnv

		wfState, err := workflow.NewState(stCtx)
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

	stCtx, wfState, err := CreateWorkflowContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	retrievedStateInterface, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
	require.NoError(t, err)
	require.NotNil(t, retrievedStateInterface)

	retrievedBaseState, ok := retrievedStateInterface.(*workflow.State)
	require.True(t, ok, "Retrieved state should be *workflow.State")

	assert.Equal(t, wfState.GetID(), retrievedBaseState.GetID())
	assert.Equal(t, nats.ComponentWorkflow, retrievedBaseState.GetID().Component)
	assert.Equal(t, nats.StatusPending, retrievedBaseState.GetStatus())
	assert.Equal(t, stCtx.GetWorkflowExecID(), retrievedBaseState.GetID().ExecID)
	assert.Equal(t, *wfState.GetEnv(), *retrievedBaseState.GetEnv())
	assert.Equal(t, *wfState.GetTrigger(), *retrievedBaseState.GetTrigger())
	assert.Equal(t, *wfState.GetOutput(), *retrievedBaseState.GetOutput())
}

func TestWorkflowStateUpdates(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, wfState, err := CreateWorkflowContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	wfState.SetStatus(nats.StatusRunning)
	updatedOutput := &common.Output{
		"result": "processing",
	}
	wfState.Output = updatedOutput // Direct assignment for BaseState.Output

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	retrievedStateInterface, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
	require.NoError(t, err)
	require.NotNil(t, retrievedStateInterface)

	retrievedBaseState, ok := retrievedStateInterface.(*workflow.State)
	require.True(t, ok, "Retrieved state should be *workflow.State")

	assert.Equal(t, nats.StatusRunning, retrievedBaseState.GetStatus())
	require.NotNil(t, retrievedBaseState.GetOutput())
	assert.Equal(t, *updatedOutput, *retrievedBaseState.GetOutput())
	assert.Equal(t, *wfState.GetEnv(), *retrievedBaseState.GetEnv()) // Ensure other fields remain consistent
}

func TestWorkflowStateUpdateFromEvent(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, wfState, err := CreateWorkflowContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)
	assert.Equal(t, nats.StatusPending, wfState.Status)

	t.Run("Should update status to Running when receiving EventWorkflowStarted", func(t *testing.T) {
		metadata := CreateWorkflowEventMetadata(
			stCtx.GetCorrID().String(),
			stCtx.GetWorkflowExecID().String(),
			stCtx.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowStarted{
			Metadata: metadata,
			Details: &pb.EventWorkflowStarted_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, wfState.Status)

		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update status to Paused when receiving WorkflowExecutionPausedEvent", func(t *testing.T) {
		metadata := CreateWorkflowEventMetadata(
			stCtx.GetCorrID().String(),
			stCtx.GetWorkflowExecID().String(),
			stCtx.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowPaused{
			Metadata: metadata,
			Details: &pb.EventWorkflowPaused_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_PAUSED,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusPaused, wfState.Status)

		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusPaused, retrievedState.GetStatus())
	})

	t.Run("Should update status back to Running when receiving WorkflowExecutionResumedEvent", func(t *testing.T) {
		metadata := CreateWorkflowEventMetadata(
			stCtx.GetCorrID().String(),
			stCtx.GetWorkflowExecID().String(),
			stCtx.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowResumed{
			Metadata: metadata,
			Details: &pb.EventWorkflowResumed_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, wfState.Status)

		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update status to Success when receiving WorkflowExecutionSuccessEvent", func(t *testing.T) {
		metadata := CreateWorkflowEventMetadata(
			stCtx.GetCorrID().String(),
			stCtx.GetWorkflowExecID().String(),
			stCtx.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowSuccess{
			Metadata: metadata,
			Details: &pb.EventWorkflowSuccess_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, wfState.Status)

		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.GetCorrID(), stCtx.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())
	})

	t.Run("Should update both status and output when receiving WorkflowExecutionSuccessEvent with Result", func(t *testing.T) {
		newExec, newWfState, err := CreateWorkflowContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newWfState)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		resultData, err := CreateSuccessResult("Workflow completed successfully", 42, map[string]any{
			"details": map[string]interface{}{
				"duration": 1500,
				"steps":    3,
			},
		})
		require.NoError(t, err)

		metadata := CreateWorkflowEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowSuccess{
			Metadata: metadata,
			Details: &pb.EventWorkflowSuccess_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
				Result: resultData,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusSuccess, newWfState.Status)

		assert.Nil(t, newWfState.Error, "Error should be nil on success")

		require.NotNil(t, newWfState.Output)
		assert.Equal(t, "Workflow completed successfully", (*newWfState.Output)["message"])
		assert.Equal(t, float64(42), (*newWfState.Output)["count"])

		details, ok := (*newWfState.Output)["details"].(map[string]interface{})
		require.True(t, ok, "details should be a map")
		assert.Equal(t, float64(1500), details["duration"])
		assert.Equal(t, float64(3), details["steps"])

		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.GetCorrID(), newExec.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())

		output := retrievedState.GetOutput()
		require.NotNil(t, output)
		assert.Equal(t, "Workflow completed successfully", (*output)["message"])
	})

	t.Run("Should update status to Failed when receiving EventWorkflowFailed", func(t *testing.T) {
		newExec, newWfState, err := CreateWorkflowContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newWfState)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		metadata := CreateWorkflowEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowFailed{
			Metadata: metadata,
			Details: &pb.EventWorkflowFailed_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, newWfState.Status)

		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.GetCorrID(), newExec.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())
	})

	t.Run("Should update both status and error output when receiving EventWorkflowFailed with Error", func(t *testing.T) {
		newExec, newWfState, err := CreateWorkflowContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newWfState)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("Workflow execution failed", "ERR_WORKFLOW_FAILED", map[string]any{
			"line":     42,
			"file":     "workflow.go",
			"function": "ProcessData",
		})
		require.NoError(t, err)

		metadata := CreateWorkflowEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowFailed{
			Metadata: metadata,
			Details: &pb.EventWorkflowFailed_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_FAILED,
				Error:  errorResult,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusFailed, newWfState.Status)

		require.NotNil(t, newWfState.Error)
		assert.Equal(t, "Workflow execution failed", newWfState.Error.Message)
		assert.Equal(t, "ERR_WORKFLOW_FAILED", newWfState.Error.Code)

		require.NotNil(t, newWfState.Error.Details)
		assert.Equal(t, float64(42), newWfState.Error.Details["line"])
		assert.Equal(t, "workflow.go", newWfState.Error.Details["file"])
		assert.Equal(t, "ProcessData", newWfState.Error.Details["function"])

		assert.Nil(t, newWfState.Output)

		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.GetCorrID(), newExec.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())

		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Workflow execution failed", errRes.Message)
		assert.Equal(t, "ERR_WORKFLOW_FAILED", errRes.Code)
		require.NotNil(t, errRes.Details)
		assert.Equal(t, float64(42), errRes.Details["line"])
	})

	t.Run("Should update status to Canceled when receiving WorkflowExecutionCanceledEvent", func(t *testing.T) {
		newExec, newWfState, err := CreateWorkflowContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newWfState)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		metadata := CreateWorkflowEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowCanceled{
			Metadata: metadata,
			Details: &pb.EventWorkflowCanceled_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_CANCELED,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusCanceled, newWfState.Status)

		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.GetCorrID(), newExec.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusCanceled, retrievedState.GetStatus())
	})

	t.Run("Should update status to TimedOut when receiving WorkflowExecutionTimedOutEvent", func(t *testing.T) {
		newExec, newWfState, err := CreateWorkflowContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newWfState)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("Workflow execution timed out", "ERR_WORKFLOW_TIMEOUT", map[string]any{
			"timeout": 30,
			"unit":    "seconds",
		})
		require.NoError(t, err)

		metadata := CreateWorkflowEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetWorkflowStateID().String(),
		)
		event := &pb.EventWorkflowTimedOut{
			Metadata: metadata,
			Details: &pb.EventWorkflowTimedOut_Details{
				Status: pb.WorkflowStatus_WORKFLOW_STATUS_TIMED_OUT,
				Error:  errorResult,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusTimedOut, newWfState.Status)

		require.NotNil(t, newWfState.Error)
		assert.Equal(t, "Workflow execution timed out", newWfState.Error.Message)
		assert.Equal(t, "ERR_WORKFLOW_TIMEOUT", newWfState.Error.Code)

		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.GetCorrID(), newExec.GetWorkflowExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusTimedOut, retrievedState.GetStatus())

		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Workflow execution timed out", errRes.Message)
	})

	t.Run("Should return error when receiving unsupported event type", func(t *testing.T) {
		unsupportedEvent := struct{}{}
		err := wfState.UpdateFromEvent(unsupportedEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported event type")
	})
}
