package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestToolStateInitialization(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTool})
	defer tb.Cleanup()

	stCtx, toolState, err := CreateToolContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx, "Tool execution should not be nil")
	require.NotNil(t, toolState)

	corrID := stCtx.GetCorrID()
	workflowExecID := stCtx.GetWorkflowExecID()
	taskExecID := stCtx.GetTaskExecID()
	toolExecID := stCtx.GetToolExecID()

	t.Run("Should correctly initialize IDs and default status", func(t *testing.T) {
		assert.Equal(t, corrID.String(), toolState.GetID().CorrID.String())
		assert.Equal(t, workflowExecID.String(), stCtx.GetWorkflowExecID().String())
		assert.Equal(t, taskExecID.String(), stCtx.GetTaskExecID().String())
		assert.Equal(t, toolExecID.String(), stCtx.GetToolExecID().String())
		assert.Equal(t, toolExecID.String(), toolState.GetID().ExecID.String())
		assert.Equal(t, nats.StatusPending, toolState.GetStatus())
	})

	t.Run("Should merge environments with tool env taking precedence and resolve templates", func(t *testing.T) {
		expectedEnv := common.EnvMap{
			"TASK_KEY":     "task_val",
			"TOOL_KEY":     "tool_val",
			"OVERRIDE_KEY": "tool_override",
			"SHARED_ENV":   "from_task_env",
			"FROM_TRIGGER": "trigger_data_value",
			"FROM_INPUT":   "tool_input_value",
			"FROM_ENV":     "from_task_env",
		}
		require.NotNil(t, toolState.GetEnv(), "Env should not be nil")
		assert.Equal(t, expectedEnv, *toolState.GetEnv())
	})

	t.Run("Should merge inputs with task input taking precedence and resolve templates", func(t *testing.T) {
		expectedInput := common.Input{
			"task_param":          "task_input_value",
			"tool_param":          "tool_input_value",
			"COMMON_PARAM":        "task_common_val",
			"TEMPLATE_PARAM":      "trigger_data_value",
			"TOOL_TEMPLATE_PARAM": "trigger_data_value",
		}
		require.NotNil(t, toolState.GetInput(), "Input should not be nil")
		assert.Equal(t, expectedInput, *toolState.GetInput())
	})

	t.Run("Should correctly initialize Trigger and Output", func(t *testing.T) {
		require.NotNil(t, toolState.GetTrigger(), "Trigger should not be nil")
		expectedTrigger := common.Input{
			"data": map[string]any{
				"value": "trigger_data_value",
			},
		}
		assert.Equal(t, expectedTrigger, *toolState.GetTrigger())

		require.NotNil(t, toolState.GetOutput(), "Output should be initialized")
		assert.Empty(t, *toolState.GetOutput(), "Output should be empty initially")
	})
}

func TestToolStatePersistence(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTool})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	t.Run("Should persist state and allow accurate retrieval", func(t *testing.T) {
		stCtx, originalToolState, err := CreateToolContextAndState()
		require.NoError(t, err)
		require.NotNil(t, stCtx)
		require.NotNil(t, originalToolState)
		toolExecID := stCtx.GetToolExecID()

		err = stateManager.SaveState(originalToolState)
		require.NoError(t, err)

		retrievedStateInterface, err := stateManager.GetToolState(stCtx.GetCorrID(), toolExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)

		retrievedBaseState, ok := retrievedStateInterface.(*tool.State)
		require.True(t, ok, "Retrieved state should be of type *state.BaseState")

		assert.Equal(t, originalToolState.GetID(), retrievedBaseState.GetID())
		assert.Equal(t, nats.ComponentTool, retrievedBaseState.GetID().Component)
		assert.Equal(t, originalToolState.GetStatus(), retrievedBaseState.GetStatus())
		assert.Equal(t, *originalToolState.GetEnv(), *retrievedBaseState.GetEnv())
		assert.Equal(t, *originalToolState.GetTrigger(), *retrievedBaseState.GetTrigger())
		assert.Equal(t, *originalToolState.GetInput(), *retrievedBaseState.GetInput())
		assert.Equal(t, *originalToolState.GetOutput(), *retrievedBaseState.GetOutput())
	})
}

func TestToolStateUpdates(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTool})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	t.Run("Should reflect updates to status and output after saving", func(t *testing.T) {
		stCtx, toolStateInstance, err := CreateToolContextAndState()
		require.NoError(t, err)
		require.NotNil(t, stCtx)
		require.NotNil(t, toolStateInstance)
		toolExecID := stCtx.GetToolExecID()

		err = stateManager.SaveState(toolStateInstance)
		require.NoError(t, err)

		toolStateInstance.SetStatus(nats.StatusSuccess)
		newOutputData := common.Output{"result": "tool_finished", "value": 99.9}
		toolStateInstance.Output = &newOutputData

		err = stateManager.SaveState(toolStateInstance)
		require.NoError(t, err)

		retrievedStateInterface, err := stateManager.GetToolState(stCtx.GetCorrID(), toolExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)

		retrievedBaseState, ok := retrievedStateInterface.(*tool.State)
		require.True(t, ok, "Retrieved state should be of type *state.BaseState")

		assert.Equal(t, nats.StatusSuccess, retrievedBaseState.GetStatus())
		require.NotNil(t, retrievedBaseState.GetOutput())
		assert.Equal(t, newOutputData, *retrievedBaseState.GetOutput())
		assert.Equal(t, *toolStateInstance.GetEnv(), *retrievedBaseState.GetEnv())
	})
}

func TestToolStateUpdateFromEvent(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentTool})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	stCtx, toolState, err := CreateToolContextAndState()
	require.NoError(t, err)
	require.NotNil(t, stCtx)
	require.NotNil(t, toolState)

	err = stateManager.SaveState(toolState)
	require.NoError(t, err)
	assert.Equal(t, nats.StatusPending, toolState.Status)

	corrID := stCtx.GetCorrID()
	workflowExecID := stCtx.GetWorkflowExecID()
	taskExecID := stCtx.GetTaskExecID()

	t.Run("Should update status to Running when receiving ToolExecutionStartedEvent", func(t *testing.T) {
		event := &pb.EventToolStarted{
			Metadata: &pb.ToolMetadata{
				Source:          "test",
				CorrelationId:   corrID.String(),
				WorkflowId:      "workflow-id",
				WorkflowExecId:  workflowExecID.String(),
				WorkflowStateId: stCtx.GetWorkflowStateID().String(),
				TaskId:          "task-id",
				TaskExecId:      taskExecID.String(),
				TaskStateId:     stCtx.GetTaskStateID().String(),
				ToolId:          "tool-id",
				ToolExecId:      stCtx.GetToolExecID().String(),
				ToolStateId:     stCtx.GetToolStateID().String(),
				Time:            timepb.Now(),
				Subject:         "",
			},
		}

		err := toolState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, toolState.Status)

		err = stateManager.SaveState(toolState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetToolState(corrID, stCtx.GetToolExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update both status and output when receiving ToolExecutionSuccessEvent with Result", func(t *testing.T) {
		resultData, err := CreateSuccessResult("Tool executed successfully", 42, map[string]any{
			"files": []interface{}{"file1.txt", "file2.txt"},
			"details": map[string]interface{}{
				"duration": 1200,
				"command":  "ls -la",
			},
		})
		require.NoError(t, err)

		event := &pb.EventToolSuccess{
			Metadata: &pb.ToolMetadata{
				Source:          "test",
				CorrelationId:   corrID.String(),
				WorkflowId:      "workflow-id",
				WorkflowExecId:  workflowExecID.String(),
				WorkflowStateId: stCtx.GetWorkflowStateID().String(),
				TaskId:          "task-id",
				TaskExecId:      taskExecID.String(),
				TaskStateId:     stCtx.GetTaskStateID().String(),
				ToolId:          "tool-id",
				ToolExecId:      stCtx.GetToolExecID().String(),
				ToolStateId:     stCtx.GetToolStateID().String(),
				Time:            timepb.Now(),
				Subject:         "",
			},
			Details: &pb.EventToolSuccess_Details{
				Result: resultData,
			},
		}

		err = toolState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusSuccess, toolState.Status)

		require.NotNil(t, toolState.Output)
		assert.Equal(t, "Tool executed successfully", (*toolState.Output)["message"])
		assert.Equal(t, float64(42), (*toolState.Output)["count"])

		files, ok := (*toolState.Output)["files"].([]interface{})
		require.True(t, ok, "files should be an array")
		require.Len(t, files, 2)
		assert.Equal(t, "file1.txt", files[0])
		assert.Equal(t, "file2.txt", files[1])

		details, ok := (*toolState.Output)["details"].(map[string]interface{})
		require.True(t, ok, "details should be a map")
		assert.Equal(t, float64(1200), details["duration"])
		assert.Equal(t, "ls -la", details["command"])

		err = stateManager.SaveState(toolState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetToolState(corrID, stCtx.GetToolExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())

		output := retrievedState.GetOutput()
		require.NotNil(t, output)
		assert.Equal(t, "Tool executed successfully", (*output)["message"])
	})

	t.Run("Should update both status and error output when receiving ToolExecutionFailedEvent with Error", func(t *testing.T) {
		newExec, newToolState, err := CreateToolContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newToolState)

		err = stateManager.SaveState(newToolState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("Tool execution failed", "ERR_TOOL_FAILED", map[string]any{
			"command":   "git clone",
			"exit_code": 128,
			"stderr":    "fatal: repository not found",
		})
		require.NoError(t, err)

		event := &pb.EventToolFailed{
			Metadata: &pb.ToolMetadata{
				Source:          "test",
				CorrelationId:   newExec.GetCorrID().String(),
				WorkflowId:      "workflow-id",
				WorkflowExecId:  newExec.GetWorkflowExecID().String(),
				WorkflowStateId: newExec.GetWorkflowStateID().String(),
				TaskId:          "task-id",
				TaskExecId:      newExec.GetTaskExecID().String(),
				TaskStateId:     newExec.GetTaskStateID().String(),
				ToolId:          "tool-id",
				ToolExecId:      newExec.GetToolExecID().String(),
				ToolStateId:     newExec.GetToolStateID().String(),
				Time:            timepb.Now(),
				Subject:         "",
			},
			Details: &pb.EventToolFailed_Details{
				Error: errorResult,
			},
		}

		err = newToolState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusFailed, newToolState.Status)

		require.NotNil(t, newToolState.Error)
		assert.Equal(t, "Tool execution failed", newToolState.Error.Message)
		assert.Equal(t, "ERR_TOOL_FAILED", newToolState.Error.Code)

		require.NotNil(t, newToolState.Error.Details)
		assert.Equal(t, "git clone", newToolState.Error.Details["command"])
		assert.Equal(t, float64(128), newToolState.Error.Details["exit_code"])
		assert.Equal(t, "fatal: repository not found", newToolState.Error.Details["stderr"])

		assert.Nil(t, newToolState.Output)

		err = stateManager.SaveState(newToolState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetToolState(newExec.GetCorrID(), newExec.GetToolExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())

		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Tool execution failed", errRes.Message)
		assert.Equal(t, "ERR_TOOL_FAILED", errRes.Code)
	})

	t.Run("Should return error when receiving unsupported event type", func(t *testing.T) {
		unsupportedEvent := struct{}{}
		err := toolState.UpdateFromEvent(unsupportedEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported event type")
	})
}
