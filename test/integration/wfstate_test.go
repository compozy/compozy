package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbworkflow "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	structpb "google.golang.org/protobuf/types/known/structpb"
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
	stCtx, err := workflow.NewContext(triggerInput, projectEnv)
	require.NoError(t, err)
	require.NotNil(t, stCtx)

	t.Run("Should correctly initialize IDs, status, and basic fields", func(t *testing.T) {
		assert.NotEmpty(t, stCtx.CorrID)
		assert.NotEmpty(t, stCtx.WorkflowExecID)
		assert.Equal(t, triggerInput, stCtx.TriggerInput)
		assert.Equal(t, projectEnv, stCtx.ProjectEnv)

		stCtx.WorkflowEnv = common.EnvMap{
			"WORKFLOW_ENV": "workflow_value",
		}

		wfState, err := workflow.NewState(stCtx)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		assert.Equal(t, nats.StatusPending, wfState.Status)
		assert.Equal(t, stCtx.WorkflowExecID, wfState.GetContext().WorkflowExecID)
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
		stCtx, err := workflow.NewContext(triggerInput, projectEnv)
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

	triggerInput := &common.Input{
		"request_id": "test-123",
		"payload": map[string]any{
			"message": "Hello, world!",
		},
	}
	projectEnv := common.EnvMap{
		"ENV_VAR1": "value1",
	}
	stCtx, err := workflow.NewContext(triggerInput, projectEnv)
	require.NoError(t, err)
	require.NotNil(t, stCtx)

	wfState, err := workflow.NewState(stCtx)
	require.NoError(t, err)
	require.NotNil(t, wfState)

	err = stateManager.SaveState(wfState)
	require.NoError(t, err)

	retrievedStateInterface, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, retrievedStateInterface)

	retrievedBaseState, ok := retrievedStateInterface.(*workflow.State)
	require.True(t, ok, "Retrieved state should be *workflow.State")

	assert.Equal(t, wfState.GetID(), retrievedBaseState.GetID())
	assert.Equal(t, nats.ComponentWorkflow, retrievedBaseState.GetID().Component)
	assert.Equal(t, nats.StatusPending, retrievedBaseState.GetStatus())
	assert.Equal(t, stCtx.WorkflowExecID, retrievedBaseState.GetID().ExecID)
	assert.Equal(t, *wfState.GetEnv(), *retrievedBaseState.GetEnv())
	assert.Equal(t, *wfState.GetTrigger(), *retrievedBaseState.GetTrigger())
	assert.Equal(t, *wfState.GetOutput(), *retrievedBaseState.GetOutput())
}

func TestWorkflowStateUpdates(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
	require.NoError(t, err)
	require.NotNil(t, stCtx)

	wfState, err := workflow.NewState(stCtx)
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

	retrievedStateInterface, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
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
	// Setup
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentWorkflow})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
	require.NoError(t, err)
	require.NotNil(t, stCtx)

	wfState, err := workflow.NewState(stCtx)
	require.NoError(t, err)
	require.NotNil(t, wfState)

	// Save initial state
	err = stateManager.SaveState(wfState)
	require.NoError(t, err)
	assert.Equal(t, nats.StatusPending, wfState.Status)

	// Test cases for each event type
	t.Run("Should update status to Running when receiving WorkflowExecutionStartedEvent", func(t *testing.T) {
		event := &pbworkflow.WorkflowExecutionStartedEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(stCtx.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(stCtx.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionStartedEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, wfState.Status)

		// Save and verify
		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update status to Paused when receiving WorkflowExecutionPausedEvent", func(t *testing.T) {
		event := &pbworkflow.WorkflowExecutionPausedEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(stCtx.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(stCtx.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionPausedEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_PAUSED,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusPaused, wfState.Status)

		// Save and verify
		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusPaused, retrievedState.GetStatus())
	})

	t.Run("Should update status back to Running when receiving WorkflowExecutionResumedEvent", func(t *testing.T) {
		event := &pbworkflow.WorkflowExecutionResumedEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(stCtx.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(stCtx.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionResumedEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, wfState.Status)

		// Save and verify
		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update status to Success when receiving WorkflowExecutionSuccessEvent", func(t *testing.T) {
		event := &pbworkflow.WorkflowExecutionSuccessEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(stCtx.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(stCtx.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionSuccessEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
			},
		}

		err := wfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, wfState.Status)

		// Save and verify
		err = stateManager.SaveState(wfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(stCtx.CorrID, stCtx.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())
	})

	t.Run("Should update both status and output when receiving WorkflowExecutionSuccessEvent with Result", func(t *testing.T) {
		// Create a new workflow state for this test
		newExec, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
		require.NoError(t, err)
		newWfState, err := workflow.NewState(newExec)
		require.NoError(t, err)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		// Create a structpb.Struct with test data
		resultData, err := structpb.NewStruct(map[string]interface{}{
			"message": "Workflow completed successfully",
			"count":   42,
			"details": map[string]interface{}{
				"duration": 1500,
				"steps":    3,
			},
		})
		require.NoError(t, err)

		// Create success event with result
		event := &pbworkflow.WorkflowExecutionSuccessEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(newExec.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(newExec.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionSuccessEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_SUCCESS,
				Result: resultData,
			},
		}

		// Apply the event
		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)

		// Verify status update
		assert.Equal(t, nats.StatusSuccess, newWfState.Status)

		// Verify error is nil
		assert.Nil(t, newWfState.Error, "Error should be nil on success")

		// Verify output update
		require.NotNil(t, newWfState.Output)
		assert.Equal(t, "Workflow completed successfully", (*newWfState.Output)["message"])
		assert.Equal(t, float64(42), (*newWfState.Output)["count"])

		// Verify nested map is correctly converted
		details, ok := (*newWfState.Output)["details"].(map[string]interface{})
		require.True(t, ok, "details should be a map")
		assert.Equal(t, float64(1500), details["duration"])
		assert.Equal(t, float64(3), details["steps"])

		// Save and verify
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.CorrID, newExec.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())

		// Verify output was saved correctly
		output := retrievedState.GetOutput()
		require.NotNil(t, output)
		assert.Equal(t, "Workflow completed successfully", (*output)["message"])
	})

	t.Run("Should update status to Failed when receiving WorkflowExecutionFailedEvent", func(t *testing.T) {
		// Create a new workflow state for testing failure
		newExec, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
		require.NoError(t, err)
		newWfState, err := workflow.NewState(newExec)
		require.NoError(t, err)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		event := &pbworkflow.WorkflowExecutionFailedEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(newExec.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(newExec.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionFailedEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, newWfState.Status)

		// Save and verify
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.CorrID, newExec.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())
	})

	t.Run("Should update both status and error output when receiving WorkflowExecutionFailedEvent with Error", func(t *testing.T) {
		// Create a new workflow state for this test
		newExec, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
		require.NoError(t, err)
		newWfState, err := workflow.NewState(newExec)
		require.NoError(t, err)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		// Create error details struct
		errorDetails, err := structpb.NewStruct(map[string]interface{}{
			"line":     42,
			"file":     "workflow.go",
			"function": "ProcessData",
		})
		require.NoError(t, err)

		// Create error code value
		errorCode := "ERR_WORKFLOW_FAILED"

		// Create failed event with error result
		event := &pbworkflow.WorkflowExecutionFailedEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(newExec.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(newExec.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionFailedEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_FAILED,
				Error: &pbcommon.ErrorResult{
					Message: "Workflow execution failed",
					Code:    &errorCode,
					Details: errorDetails,
				},
			},
		}

		// Apply the event
		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)

		// Verify status update
		assert.Equal(t, nats.StatusFailed, newWfState.Status)

		// Verify error details
		require.NotNil(t, newWfState.Error)
		assert.Equal(t, "Workflow execution failed", newWfState.Error.Message)
		assert.Equal(t, errorCode, newWfState.Error.Code)

		// Verify error details are correctly stored
		require.NotNil(t, newWfState.Error.Details)
		assert.Equal(t, float64(42), newWfState.Error.Details["line"])
		assert.Equal(t, "workflow.go", newWfState.Error.Details["file"])
		assert.Equal(t, "ProcessData", newWfState.Error.Details["function"])

		// Verify output is nil when there's an error
		assert.Nil(t, newWfState.Output)

		// Save and verify
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.CorrID, newExec.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())

		// Verify error was saved correctly
		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Workflow execution failed", errRes.Message)
		assert.Equal(t, errorCode, errRes.Code)
		require.NotNil(t, errRes.Details)
		assert.Equal(t, float64(42), errRes.Details["line"])
	})

	t.Run("Should update status to Canceled when receiving WorkflowExecutionCanceledEvent", func(t *testing.T) {
		// Create a new workflow state for testing cancellation
		newExec, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
		require.NoError(t, err)
		newWfState, err := workflow.NewState(newExec)
		require.NoError(t, err)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		event := &pbworkflow.WorkflowExecutionCanceledEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(newExec.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(newExec.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionCanceledEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_CANCELED,
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusCanceled, newWfState.Status)

		// Save and verify
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.CorrID, newExec.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusCanceled, retrievedState.GetStatus())
	})

	t.Run("Should update status to TimedOut when receiving WorkflowExecutionTimedOutEvent", func(t *testing.T) {
		// Create a new workflow state for testing timeout
		newExec, err := workflow.NewContext(&common.Input{}, common.EnvMap{})
		require.NoError(t, err)
		newWfState, err := workflow.NewState(newExec)
		require.NoError(t, err)
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		// Create error for timeout
		errorCode := "ERR_WORKFLOW_TIMEOUT"
		errorDetails, err := structpb.NewStruct(map[string]interface{}{
			"timeout": 30,
			"unit":    "seconds",
		})
		require.NoError(t, err)

		event := &pbworkflow.WorkflowExecutionTimedOutEvent{
			Metadata: &pbcommon.Metadata{
				CorrelationId: string(newExec.CorrID),
			},
			Workflow: &pbcommon.WorkflowInfo{
				Id:     "workflow-id",
				ExecId: string(newExec.WorkflowExecID),
			},
			Details: &pbworkflow.WorkflowExecutionTimedOutEvent_Details{
				Status: pbworkflow.WorkflowStatus_WORKFLOW_STATUS_TIMED_OUT,
				Error: &pbcommon.ErrorResult{
					Message: "Workflow execution timed out",
					Code:    &errorCode,
					Details: errorDetails,
				},
			},
		}

		err = newWfState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusTimedOut, newWfState.Status)

		// Verify error details
		require.NotNil(t, newWfState.Error)
		assert.Equal(t, "Workflow execution timed out", newWfState.Error.Message)
		assert.Equal(t, errorCode, newWfState.Error.Code)

		// Save and verify
		err = stateManager.SaveState(newWfState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetWorkflowState(newExec.CorrID, newExec.WorkflowExecID)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusTimedOut, retrievedState.GetStatus())

		// Verify error was saved correctly
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
