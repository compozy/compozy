package test

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentStateInitialization(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentAgent})
	defer tb.Cleanup()

	ctx, agentState, err := CreateAgentContextAndState()
	require.NoError(t, err)
	require.NotNil(t, ctx, "Agent execution should not be nil")
	require.NotNil(t, agentState)

	corrID := ctx.GetCorrID()
	workflowExecID := ctx.GetWorkflowExecID()
	taskExecID := ctx.GetTaskExecID()
	agentExecID := ctx.GetAgentExecID()

	t.Run("Should correctly initialize IDs and default status", func(t *testing.T) {
		assert.Equal(t, corrID, agentState.GetID().CorrID)
		assert.Equal(t, workflowExecID, agentState.Context.GetWorkflowExecID())
		assert.Equal(t, taskExecID, agentState.Context.GetTaskExecID())
		assert.Equal(t, agentExecID, agentState.Context.GetAgentExecID())
		assert.Equal(t, agentExecID, agentState.GetID().ExecID)
		assert.Equal(t, nats.StatusPending, agentState.GetStatus())
	})

	t.Run("Should merge environments with agent env taking precedence and resolve templates", func(t *testing.T) {
		expectedEnv := common.EnvMap{
			"TASK_KEY":     "task_val",
			"AGENT_KEY":    "agent_val",
			"OVERRIDE_KEY": "agent_override",
			"SHARED_ENV":   "from_task_env",
			"FROM_TRIGGER": "trigger_data_value",
			"FROM_INPUT":   "agent_input_value",
			"FROM_ENV":     "from_task_env",
		}
		require.NotNil(t, agentState.GetEnv(), "Env should not be nil")
		assert.Equal(t, expectedEnv, *agentState.GetEnv())
	})

	t.Run("Should merge inputs with task input taking precedence and resolve templates", func(t *testing.T) {
		expectedInput := common.Input{
			"task_param":           "task_input_value",
			"agent_param":          "agent_input_value",
			"COMMON_PARAM":         "task_common_val",
			"TEMPLATE_PARAM":       "trigger_data_value",
			"AGENT_TEMPLATE_PARAM": "trigger_data_value",
		}
		require.NotNil(t, agentState.GetInput(), "Input should not be nil")
		assert.Equal(t, expectedInput, *agentState.GetInput())
	})

	t.Run("Should correctly initialize Trigger and Output", func(t *testing.T) {
		require.NotNil(t, agentState.GetTrigger(), "Trigger should not be nil")
		expectedTrigger := common.Input{
			"data": map[string]any{
				"value": "trigger_data_value",
			},
		}
		assert.Equal(t, expectedTrigger, *agentState.GetTrigger())

		require.NotNil(t, agentState.GetOutput(), "Output should be initialized")
		assert.Empty(t, *agentState.GetOutput(), "Output should be empty initially")
	})
}

func TestAgentStatePersistence(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentAgent})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	t.Run("Should persist state and allow accurate retrieval", func(t *testing.T) {
		ctx, originalAgentState, err := CreateAgentContextAndState()
		require.NoError(t, err)
		require.NotNil(t, ctx)
		require.NotNil(t, originalAgentState)
		agentExecID := ctx.GetAgentExecID()

		err = stateManager.SaveState(originalAgentState)
		require.NoError(t, err)

		retrievedStateInterface, err := stateManager.GetAgentState(ctx.GetCorrID(), agentExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)

		retrievedBaseState, ok := retrievedStateInterface.(*agent.State)
		require.True(t, ok, "Retrieved state should be of type *state.BaseState")

		assert.Equal(t, originalAgentState.GetID(), retrievedBaseState.GetID())
		assert.Equal(t, nats.ComponentAgent, retrievedBaseState.GetID().Component)
		assert.Equal(t, originalAgentState.GetStatus(), retrievedBaseState.GetStatus())
		assert.Equal(t, *originalAgentState.GetEnv(), *retrievedBaseState.GetEnv())
		assert.Equal(t, *originalAgentState.GetTrigger(), *retrievedBaseState.GetTrigger())
		assert.Equal(t, *originalAgentState.GetInput(), *retrievedBaseState.GetInput())
		assert.Equal(t, *originalAgentState.GetOutput(), *retrievedBaseState.GetOutput())
	})
}

func TestAgentStateUpdates(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentAgent})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	t.Run("Should reflect updates to status and output after saving", func(t *testing.T) {
		ctx, agentStateInstance, err := CreateAgentContextAndState()
		require.NoError(t, err)
		require.NotNil(t, ctx)
		require.NotNil(t, agentStateInstance)
		agentExecID := ctx.GetAgentExecID()

		err = stateManager.SaveState(agentStateInstance)
		require.NoError(t, err)

		agentStateInstance.SetStatus(nats.StatusSuccess)
		newOutputData := common.Output{
			"result": "agent_done",
			"detail": "all good",
		}
		agentStateInstance.Output = &newOutputData

		err = stateManager.SaveState(agentStateInstance)
		require.NoError(t, err)

		retrievedStateInterface, err := stateManager.GetAgentState(ctx.GetCorrID(), agentExecID)
		require.NoError(t, err)
		require.NotNil(t, retrievedStateInterface)

		retrievedBaseState, ok := retrievedStateInterface.(*agent.State)
		require.True(t, ok, "Retrieved state should be of type *state.BaseState")

		assert.Equal(t, nats.StatusSuccess, retrievedBaseState.GetStatus())
		require.NotNil(t, retrievedBaseState.GetOutput())
		assert.Equal(t, newOutputData, *retrievedBaseState.GetOutput())
		assert.Equal(t, *agentStateInstance.GetEnv(), *retrievedBaseState.GetEnv())
	})
}

func TestAgentStateUpdateFromEvent(t *testing.T) {
	tb := SetupIntegrationTestBed(t, DefaultTestTimeout, []nats.ComponentType{nats.ComponentAgent})
	defer tb.Cleanup()
	stateManager := tb.StateManager

	ctx, agentState, err := CreateAgentContextAndState()
	require.NoError(t, err)
	require.NotNil(t, ctx)
	require.NotNil(t, agentState)

	err = stateManager.SaveState(agentState)
	require.NoError(t, err)
	assert.Equal(t, nats.StatusPending, agentState.Status)

	corrID := ctx.GetCorrID()
	workflowExecID := ctx.GetWorkflowExecID()
	taskExecID := ctx.GetTaskExecID()

	t.Run("Should update status to Running when receiving AgentExecutionStartedEvent", func(t *testing.T) {
		metadata := CreateAgentEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			taskExecID.String(),
			ctx.GetAgentExecID().String(),
			ctx.GetWorkflowStateID().String(),
			ctx.GetTaskStateID().String(),
			ctx.GetAgentStateID().String(),
		)
		event := CreateAgentStartedEvent(metadata)

		err := agentState.UpdateFromEvent(event)
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, agentState.Status)

		err = stateManager.SaveState(agentState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetAgentState(corrID, ctx.GetAgentExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusRunning, retrievedState.GetStatus())
	})

	t.Run("Should update both status and output when receiving AgentExecutionSuccessEvent with Result", func(t *testing.T) {
		resultData, err := CreateSuccessResult("Agent completed successfully", 42, map[string]any{
			"tokens": 2500,
			"details": map[string]any{
				"model":   "gpt-4",
				"latency": 1200,
			},
		})
		require.NoError(t, err)

		metadata := CreateAgentEventMetadata(
			corrID.String(),
			workflowExecID.String(),
			taskExecID.String(),
			ctx.GetAgentExecID().String(),
			ctx.GetWorkflowStateID().String(),
			ctx.GetTaskStateID().String(),
			ctx.GetAgentStateID().String(),
		)
		event := CreateAgentSuccessEvent(metadata, resultData)

		err = agentState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusSuccess, agentState.Status)

		require.NotNil(t, agentState.Output)
		assert.Equal(t, "Agent completed successfully", (*agentState.Output)["message"])
		assert.Equal(t, float64(42), (*agentState.Output)["count"])
		assert.Equal(t, float64(2500), (*agentState.Output)["tokens"])

		details, ok := (*agentState.Output)["details"].(map[string]interface{})
		require.True(t, ok, "details should be a map")
		assert.Equal(t, "gpt-4", details["model"])
		assert.Equal(t, float64(1200), details["latency"])

		err = stateManager.SaveState(agentState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetAgentState(corrID, ctx.GetAgentExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusSuccess, retrievedState.GetStatus())

		output := retrievedState.GetOutput()
		require.NotNil(t, output)
		assert.Equal(t, "Agent completed successfully", (*output)["message"])
	})

	t.Run("Should update both status and error output when receiving AgentExecutionFailedEvent with Error", func(t *testing.T) {
		newExec, newAgentState, err := CreateAgentContextAndState()
		require.NoError(t, err)
		require.NotNil(t, newExec)
		require.NotNil(t, newAgentState)
		err = stateManager.SaveState(newAgentState)
		require.NoError(t, err)

		errorResult, err := CreateErrorResult("Agent execution failed", "ERR_AGENT_FAILED", map[string]any{
			"model":   "gpt-4",
			"context": "LLM API call failed",
			"retry":   2,
		})
		require.NoError(t, err)

		metadata := CreateAgentEventMetadata(
			newExec.GetCorrID().String(),
			newExec.GetWorkflowExecID().String(),
			newExec.GetTaskExecID().String(),
			newExec.GetAgentExecID().String(),
			newExec.GetWorkflowStateID().String(),
			newExec.GetTaskStateID().String(),
			newExec.GetAgentStateID().String(),
		)
		event := CreateAgentFailedEvent(metadata, errorResult)

		err = newAgentState.UpdateFromEvent(event)
		require.NoError(t, err)

		assert.Equal(t, nats.StatusFailed, newAgentState.Status)

		require.NotNil(t, newAgentState.Error)
		assert.Equal(t, "Agent execution failed", newAgentState.Error.Message)
		assert.Equal(t, "ERR_AGENT_FAILED", newAgentState.Error.Code)

		require.NotNil(t, newAgentState.Error.Details)
		assert.Equal(t, "gpt-4", newAgentState.Error.Details["model"])
		assert.Equal(t, "LLM API call failed", newAgentState.Error.Details["context"])
		assert.Equal(t, float64(2), newAgentState.Error.Details["retry"])

		assert.Nil(t, newAgentState.Output)

		err = stateManager.SaveState(newAgentState)
		require.NoError(t, err)

		retrievedState, err := stateManager.GetAgentState(newExec.GetCorrID(), newExec.GetAgentExecID())
		require.NoError(t, err)
		assert.Equal(t, nats.StatusFailed, retrievedState.GetStatus())

		errRes := retrievedState.GetError()
		require.NotNil(t, errRes)
		assert.Equal(t, "Agent execution failed", errRes.Message)
		assert.Equal(t, "ERR_AGENT_FAILED", errRes.Code)
	})

	t.Run("Should return error when receiving unsupported event type", func(t *testing.T) {
		unsupportedEvent := struct{}{}
		err := agentState.UpdateFromEvent(unsupportedEvent)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported event type")
	})
}
