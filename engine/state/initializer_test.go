package state

import (
	"testing"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWorkflowState(
	wfID string,
	execID string,
	tgInput common.Input,
	projectEnv common.EnvMap,
	workflowEnv common.EnvMap,
) (State, error) {
	initializer := &WorkflowStateInitializer{
		CommonInitializer: NewCommonInitializer(),
		WorkflowID:        wfID,
		ExecID:            execID,
		TriggerInput:      tgInput,
		ProjectEnv:        projectEnv,
		WorkflowEnv:       workflowEnv,
	}
	return initializer.Initialize()
}

func newTaskState(
	tID string,
	execID string,
	wfExecID string,
	tgInput common.Input,
	workflowEnv common.EnvMap,
	taskEnv common.EnvMap,
) (State, error) {
	initializer := &TaskStateInitializer{
		CommonInitializer: NewCommonInitializer(),
		TaskID:            tID,
		ExecID:            execID,
		WorkflowExecID:    wfExecID,
		TriggerInput:      tgInput,
		WorkflowEnv:       workflowEnv,
		TaskEnv:           taskEnv,
	}
	return initializer.Initialize()
}

func newAgentState(
	agID string,
	execID string,
	tExecID string,
	wfExecID string,
	tgInput common.Input,
	taskEnv common.EnvMap,
	agentEnv common.EnvMap,
) (State, error) {
	initializer := &AgentStateInitializer{
		CommonInitializer: NewCommonInitializer(),
		AgentID:           agID,
		ExecID:            execID,
		TaskExecID:        tExecID,
		WorkflowExecID:    wfExecID,
		TriggerInput:      tgInput,
		TaskEnv:           taskEnv,
		AgentEnv:          agentEnv,
	}
	return initializer.Initialize()
}

func newToolState(
	toolID string,
	execID string,
	tExecID string,
	wfExecID string,
	tgInput common.Input,
	taskEnv common.EnvMap,
	toolEnv common.EnvMap,
) (State, error) {
	initializer := &ToolStateInitializer{
		CommonInitializer: NewCommonInitializer(),
		ToolID:            toolID,
		ExecID:            execID,
		TaskExecID:        tExecID,
		WorkflowExecID:    wfExecID,
		TriggerInput:      tgInput,
		TaskEnv:           taskEnv,
		ToolEnv:           toolEnv,
	}
	return initializer.Initialize()
}

func TestStateInitialization(t *testing.T) {
	// Test data
	tgInput := common.Input{
		"name":  "John",
		"email": "john@example.com",
	}
	projectEnv := common.EnvMap{
		"PROJECT_VERSION": "1.0.0",
		"SHARED_ENV":      "project_value",
	}
	workflowEnv := common.EnvMap{
		"WORKFLOW_VERSION": "2.0.0",
		"SHARED_ENV":       "workflow_value", // Should override project
	}
	taskEnv := common.EnvMap{
		"TASK_VERSION": "3.0.0",
		"SHARED_ENV":   "task_value", // Should override workflow
	}
	agentEnv := common.EnvMap{
		"AGENT_VERSION": "4.0.0",
		"SHARED_ENV":    "agent_value", // Should override task
	}
	toolEnv := common.EnvMap{
		"TOOL_VERSION": "5.0.0",
		"SHARED_ENV":   "tool_value", // Should override task
	}

	t.Run("WorkflowStateInitialization", func(t *testing.T) {
		// Initialize workflow state
		wfState, err := newWorkflowState(
			"test-workflow",
			"exec123",
			tgInput,
			projectEnv,
			workflowEnv,
		)
		require.NoError(t, err)
		require.NotNil(t, wfState)

		// Verify ID and status
		assert.Equal(t, "workflow:test-workflow:exec123", wfState.GetID().String())
		assert.Equal(t, nats.StatusPending, wfState.GetStatus())

		// Verify environment variables are merged correctly (workflow overrides project)
		env := wfState.GetEnv()
		assert.Equal(t, "1.0.0", (*env)["PROJECT_VERSION"])
		assert.Equal(t, "2.0.0", (*env)["WORKFLOW_VERSION"])
		assert.Equal(t, "workflow_value", (*env)["SHARED_ENV"]) // Workflow env overrides project

		// Verify trigger input is stored in context
		bsState := wfState.(*BaseState)
		assert.Equal(t, tgInput, bsState.Trigger)
	})

	t.Run("TaskStateInitialization", func(t *testing.T) {
		// Initialize task state
		taskState, err := newTaskState(
			"test-task",
			"task-exec123",
			"workflow-exec123",
			tgInput,
			workflowEnv,
			taskEnv,
		)
		require.NoError(t, err)
		require.NotNil(t, taskState)

		// Verify ID and status
		assert.Equal(t, "task:test-task:task-exec123", taskState.GetID().String())
		assert.Equal(t, nats.StatusPending, taskState.GetStatus())

		// Verify environment variables are merged correctly (task overrides workflow)
		env := taskState.GetEnv()
		assert.Equal(t, "2.0.0", (*env)["WORKFLOW_VERSION"])
		assert.Equal(t, "3.0.0", (*env)["TASK_VERSION"])
		assert.Equal(t, "task_value", (*env)["SHARED_ENV"]) // Task env overrides workflow

		// Verify trigger input is stored in context
		bsState := taskState.(*BaseState)
		assert.Equal(t, tgInput, bsState.Trigger)
	})

	t.Run("TemplateParsing", func(t *testing.T) {
		// Test data with templates
		taskEnvWithTemplates := common.EnvMap{
			"USER_NAME":  "{{ .trigger.input.name }}",
			"USER_EMAIL": "{{ .trigger.input.email }}",
		}
		inputWithTemplates := common.Input{
			"greeting":  "Hello {{ .trigger.input.name }}!",
			"signature": "Welcome to our service, {{ .env.PROJECT_VERSION }}",
			// Add nested structure with templates
			"nested": map[string]any{
				"message": "Hi {{ .trigger.input.name }}",
				"metadata": map[string]any{
					"user_email": "Contact: {{ .trigger.input.email }}",
					"version":    "v{{ .env.PROJECT_VERSION }}",
				},
			},
			// Add array with templates
			"items": []any{
				"Item 1: {{ .trigger.input.name }}",
				map[string]any{
					"label": "Item 2",
					"value": "Email: {{ .trigger.input.email }}",
				},
				[]any{
					"Subitem 1: {{ .env.PROJECT_VERSION }}",
					"Subitem 2: {{ .trigger.input.name }}",
				},
			},
		}

		// Initialize template engine
		initializer := &TaskStateInitializer{
			CommonInitializer: NewCommonInitializer(),
			TaskID:            "test-task",
			ExecID:            "task-exec123",
			WorkflowExecID:    "workflow-exec123",
			TriggerInput:      tgInput,
			WorkflowEnv:       projectEnv, // Use project env for workflow env
			TaskEnv:           taskEnvWithTemplates,
		}

		// Create the state
		state, err := initializer.Initialize()
		require.NoError(t, err)

		// Manually set input with templates
		bsState := state.(*BaseState)
		for k, v := range inputWithTemplates {
			bsState.Input[k] = v
		}

		// Parse templates
		err = initializer.Normalizer.ParseTemplates(state)
		require.NoError(t, err)

		// Verify environment variables are parsed
		env := state.GetEnv()
		assert.Equal(t, "John", (*env)["USER_NAME"])
		assert.Equal(t, "john@example.com", (*env)["USER_EMAIL"])

		// Verify input fields are parsed
		input := state.GetInput()
		assert.Equal(t, "Hello John!", (*input)["greeting"])
		assert.Equal(t, "Welcome to our service, 1.0.0", (*input)["signature"])

		// Verify nested structures are parsed
		nested, ok := (*input)["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Hi John", nested["message"])

		metadata, ok := nested["metadata"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Contact: john@example.com", metadata["user_email"])
		assert.Equal(t, "v1.0.0", metadata["version"])

		// Verify array items are parsed
		items, ok := (*input)["items"].([]any)
		require.True(t, ok)
		assert.Equal(t, "Item 1: John", items[0])

		item2, ok := items[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Email: john@example.com", item2["value"])

		subitems, ok := items[2].([]any)
		require.True(t, ok)
		assert.Equal(t, "Subitem 1: 1.0.0", subitems[0])
		assert.Equal(t, "Subitem 2: John", subitems[1])
	})

	t.Run("AgentStateInitialization", func(t *testing.T) {
		// Initialize agent state
		agState, err := newAgentState(
			"test-agent",
			"agent-exec123",
			"task-exec123",
			"workflow-exec123",
			tgInput,
			taskEnv,
			agentEnv,
		)
		require.NoError(t, err)
		require.NotNil(t, agState)

		// Verify ID and status
		assert.Equal(t, "agent:test-agent:agent-exec123", agState.GetID().String())
		assert.Equal(t, nats.StatusPending, agState.GetStatus())

		// Verify environment variables are merged correctly (agent overrides task)
		env := agState.GetEnv()
		assert.Equal(t, "3.0.0", (*env)["TASK_VERSION"])
		assert.Equal(t, "4.0.0", (*env)["AGENT_VERSION"])
		assert.Equal(t, "agent_value", (*env)["SHARED_ENV"]) // Agent env overrides task

		// Verify trigger input is stored in context
		bsState := agState.(*BaseState)
		assert.Equal(t, tgInput, bsState.Trigger)
	})

	t.Run("ToolStateInitialization", func(t *testing.T) {
		// Initialize tool state
		toolState, err := newToolState(
			"test-tool",
			"tool-exec123",
			"task-exec123",
			"workflow-exec123",
			tgInput,
			taskEnv,
			toolEnv,
		)
		require.NoError(t, err)
		require.NotNil(t, toolState)

		// Verify ID and status
		assert.Equal(t, "tool:test-tool:tool-exec123", toolState.GetID().String())
		assert.Equal(t, nats.StatusPending, toolState.GetStatus())

		// Verify environment variables are merged correctly (tool overrides task)
		env := toolState.GetEnv()
		assert.Equal(t, "3.0.0", (*env)["TASK_VERSION"])
		assert.Equal(t, "5.0.0", (*env)["TOOL_VERSION"])
		assert.Equal(t, "tool_value", (*env)["SHARED_ENV"]) // Tool env overrides task

		// Verify trigger input is stored in context
		bsState := toolState.(*BaseState)
		assert.Equal(t, tgInput, bsState.Trigger)
	})
}
