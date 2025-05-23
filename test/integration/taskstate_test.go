package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskStateInitialization(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTask})
	defer tb.Cleanup()

	stCtx, taskState, err := CreateTaskContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx, "Task execution should not be nil")
	require.NotNil(t, taskState)

	corrID := stCtx.GetCorrID()
	workflowExecID := stCtx.GetWorkflowExecID()
	taskExecID := stCtx.GetTaskExecID()

	t.Run("Should correctly initialize IDs and default status", func(t *testing.T) {
		assert.Equal(t, corrID.String(), taskState.GetID().CorrID.String())
		assert.Equal(t, workflowExecID.String(), stCtx.GetWorkflowExecID().String())
		assert.Equal(t, taskExecID.String(), stCtx.GetTaskExecID().String())
		assert.Equal(t, taskExecID.String(), taskState.GetID().ExecID.String())
		assert.Equal(t, nats.StatusPending, taskState.GetStatus())
	})

	t.Run("Should merge environments with task env taking precedence and resolve templates", func(t *testing.T) {
		expectedEnv := common.EnvMap{
			"WORKFLOW_KEY": "workflow_val",
			"TASK_KEY":     "task_val",
			"OVERRIDE_KEY": "task_override",
			"SHARED_ENV":   "from_task_env",
			"FROM_TRIGGER": "trigger_data_value",
			"FROM_INPUT":   "task_input_value",
			"FROM_ENV":     "from_task_env",
		}
		require.NotNil(t, taskState.GetEnv(), "Env should not be nil")
		assert.Equal(t, expectedEnv, *taskState.GetEnv())
	})

	t.Run("Should correctly initialize Input with templates resolved", func(t *testing.T) {
		expectedInput := common.Input{
			"task_param":     "task_input_value",
			"COMMON_PARAM":   "task_common_val",
			"TEMPLATE_PARAM": "trigger_data_value",
		}
		require.NotNil(t, taskState.GetInput(), "Input should not be nil")
		assert.Equal(t, expectedInput, *taskState.GetInput())
	})

	t.Run("Should correctly initialize Trigger and Output", func(t *testing.T) {
		require.NotNil(t, taskState.GetTrigger(), "Trigger should not be nil")
		expectedTrigger := common.Input{
			"data": map[string]any{
				"value": "trigger_data_value",
			},
		}
		assert.Equal(t, expectedTrigger, *taskState.GetTrigger())

		require.NotNil(t, taskState.GetOutput(), "Output should be initialized")
		assert.Empty(t, *taskState.GetOutput(), "Output should be empty initially")
	})
}

func TestTaskStateUpdateFromEvent(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTask})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, taskState, err := CreateTaskContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx)
	require.NotNil(t, taskState)

	err = stateManager.SaveState(taskState)
	require.NoError(t, err)
	assert.Equal(t, nats.StatusPending, taskState.Status)

	corrID := stCtx.GetCorrID()
	workflowExecID := stCtx.GetWorkflowExecID()

	t.Run("Should update status to Pending when receiving TaskDispatchedEvent", func(t *testing.T) {
		metadata := CreateTaskEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			stCtx.GetTaskExecID().String(),
			stCtx.GetWorkflowStateID().String(),
			stCtx.GetTaskStateID().String(),
		)
		event := &pb.EventTaskDispatched{
			Metadata: metadata,
		}

		err := taskState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusPending, taskState.Status)
	})

	t.Run("Should update status to Running when receiving TaskExecutionStartedEvent", func(t *testing.T) {
		metadata := CreateTaskEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			stCtx.GetTaskExecID().String(),
			stCtx.GetWorkflowStateID().String(),
			stCtx.GetTaskStateID().String(),
		)
		event := CreateTaskStartedEvent(metadata)

		err := taskState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, taskState.Status)

		err = stateManager.SaveState(taskState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetTaskState(corrID, stCtx.GetTaskExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update status to Waiting when receiving TaskExecutionWaitingStartedEvent", func(t *testing.T) {
		metadata := CreateTaskEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			stCtx.GetTaskExecID().String(),
			stCtx.GetWorkflowStateID().String(),
			stCtx.GetTaskStateID().String(),
		)
		event := &pb.EventTaskWaiting{
			Metadata: metadata,
		}

		err := taskState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusWaiting, taskState.Status)
	})

	t.Run("Should update status to Running when receiving TaskExecutionWaitingEndedEvent", func(t *testing.T) {
		metadata := CreateTaskEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			stCtx.GetTaskExecID().String(),
			stCtx.GetWorkflowStateID().String(),
			stCtx.GetTaskStateID().String(),
		)
		event := &pb.EventTaskWaitingEnded{
			Metadata: metadata,
		}

		err := taskState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, taskState.Status)
	})

	t.Run("Should update both status and output when receiving TaskExecutionSuccessEvent with Result", func(t *testing.T) {
		resultData, err := CreateSuccessResult("Task completed successfully", 42, map[string]any{
			"tokens": 2500,
			"details": map[string]interface{}{
				"type":    "completion",
				"latency": 500,
			},
		})
		require.NoError(t, err)

		metadata := CreateTaskEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			stCtx.GetTaskExecID().String(),
			stCtx.GetWorkflowStateID().String(),
			stCtx.GetTaskStateID().String(),
		)
		event := CreateTaskSuccessEvent(metadata, resultData)

		err = taskState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusSuccess, taskState.Status)

		require.NotNil(t, taskState.Output)
		assert.Equal(t, "Task completed successfully", (*taskState.Output)["message"])
		assert.Equal(t, float64(42), (*taskState.Output)["count"])
		assert.Equal(t, float64(2500), (*taskState.Output)["tokens"])

		details, ok := (*taskState.Output)["details"].(map[string]interface{})
		require.True(t, ok, "details should be a map")
		assert.Equal(t, "completion", details["type"])
		assert.Equal(t, float64(500), details["latency"])

		err = stateManager.SaveState(taskState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetTaskState(corrID, stCtx.GetTaskExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())

		output := retrievedState.GetOutput()
		require.NotNil(t, output)
		assert.Equal(t, "Task completed successfully", (*output)["message"])
	})

	t.Run("Should update both status and error output when receiving TaskExecutionFailedEvent with Error", func(t *testing.T) {
		newExec, newTaskState, err := CreateTaskContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newTaskState)

		err = stateManager.SaveState(newTaskState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("Task execution failed", "ERR_TASK_FAILED", map[string]any{
			"operation": "database_query",
			"context":   "connection failed",
			"retry":     false,
		})
		require.NoError(t, err)

		metadata := CreateTaskEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetTaskExecID().String(),
			newExec.GetWorkflowStateID().String(),
			newExec.GetTaskStateID().String(),
		)
		event := CreateTaskFailedEvent(metadata, errorResult)

		err = newTaskState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusFailed, newTaskState.Status)

		require.NotNil(t, newTaskState.Error)
		assert.Equal(t, "Task execution failed", newTaskState.Error.Message)
		assert.Equal(t, "ERR_TASK_FAILED", newTaskState.Error.Code)

		require.NotNil(t, newTaskState.Error.Details)
		assert.Equal(t, "database_query", newTaskState.Error.Details["operation"])
		assert.Equal(t, "connection failed", newTaskState.Error.Details["context"])
		assert.Equal(t, false, newTaskState.Error.Details["retry"])

		assert.Nil(t, newTaskState.Output)

		err = stateManager.SaveState(newTaskState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetTaskState(newExec.GetCorrID(), newExec.GetTaskExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())

		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Task execution failed", errRes.Message)
		assert.Equal(t, "ERR_TASK_FAILED", errRes.Code)
	})

	t.Run("Should update both status and output when receiving TaskExecutionWaitingTimedOutEvent with Result", func(t *testing.T) {
		newExec, newTaskState, err := CreateTaskContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newTaskState)

		err = stateManager.SaveState(newTaskState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("waiting condition not satisfied", "ERR_TASK_WAITING_TIMED_OUT", map[string]any{
			"reason":     "waiting condition not satisfied",
			"timeout_ms": 30000,
			"details":    "max retry attempts exceeded",
		})
		require.NoError(t, err)

		metadata := CreateTaskEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetTaskExecID().String(),
			newExec.GetWorkflowStateID().String(),
			newExec.GetTaskStateID().String(),
		)
		event := &pb.EventTaskWaitingTimedOut{
			Metadata: metadata,
			Details: &pb.EventTaskWaitingTimedOut_Details{
				Error: errorResult,
			},
		}

		err = newTaskState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusTimedOut, newTaskState.Status)

		require.NotNil(t, newTaskState.Error)
		assert.Equal(t, "waiting condition not satisfied", newTaskState.Error.Message)
		assert.Equal(t, "ERR_TASK_WAITING_TIMED_OUT", newTaskState.Error.Code)
		assert.Equal(t, float64(30000), newTaskState.Error.Details["timeout_ms"])
		assert.Equal(t, "max retry attempts exceeded", newTaskState.Error.Details["details"])

		err = stateManager.SaveState(newTaskState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetTaskState(newExec.GetCorrID(), newExec.GetTaskExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusTimedOut, retrievedState.GetStatus())
	})

	t.Run("Should return error when receiving unsupported event type", func(t *testing.T) {
		unsupportedEvent := struct{}{}
		err := taskState.UpdateFromEvent(unsupportedEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported event type")
	})
}
