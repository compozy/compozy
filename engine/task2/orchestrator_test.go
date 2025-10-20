package task2_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOrchestrator(ctx context.Context, t *testing.T) *task2.ConfigOrchestrator {
	t.Helper()
	factory, cleanup := setupTestFactory(ctx, t)
	t.Cleanup(cleanup)
	orchestrator, err := task2.NewConfigOrchestrator(ctx, factory)
	require.NoError(t, err)
	return orchestrator
}
func TestConfigOrchestrator_NormalizeTask(t *testing.T) {
	orchestrator := setupOrchestrator(t.Context(), t)
	t.Run("Should normalize basic task with template expressions", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"name":  "TestUser",
				"count": 5,
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "task1",
						Type: task.TaskTypeBasic,
						With: &core.Input{
							"message": "Count is {{ .workflow.input.count }}",
						},
					},
					BasicTask: task.BasicTask{
						Action: "Hello {{ .workflow.input.name }}",
					},
				},
			},
		}
		// Get task config
		taskConfig := &workflowConfig.Tasks[0]
		// Normalize the task
		err := orchestrator.NormalizeTask(t.Context(), workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)
		// Check normalized values
		assert.Equal(t, "Hello TestUser", taskConfig.Action)
		if taskConfig.With != nil {
			assert.Equal(t, "Count is 5", (*taskConfig.With)["message"])
		} else {
			t.Fatal("taskConfig.With is nil after normalization")
		}
	})
	t.Run("Should normalize parallel task with sub-tasks", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"prefix": "Task",
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "parallel1",
						Type: task.TaskTypeParallel,
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "{{ .workflow.input.prefix }}-1",
								Type: task.TaskTypeBasic,
							},
							BasicTask: task.BasicTask{
								Action: "action1",
							},
						},
						{
							BaseConfig: task.BaseConfig{
								ID:   "{{ .workflow.input.prefix }}-2",
								Type: task.TaskTypeBasic,
							},
							BasicTask: task.BasicTask{
								Action: "action2",
							},
						},
					},
				},
			},
		}
		// Get task config
		taskConfig := &workflowConfig.Tasks[0]
		// Normalize the task
		err := orchestrator.NormalizeTask(t.Context(), workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)
		// Check sub-tasks were normalized
		require.Len(t, taskConfig.Tasks, 2)
		assert.Equal(t, "Task-1", taskConfig.Tasks[0].ID)
		assert.Equal(t, "Task-2", taskConfig.Tasks[1].ID)
	})
	t.Run("Should normalize collection task configuration", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"items": []string{"item1", "item2", "item3"},
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "collection1",
						Type: task.TaskTypeCollection,
					},
					CollectionConfig: task.CollectionConfig{
						Items: "{{ .workflow.input.items }}",
					},
					Task: &task.Config{
						BaseConfig: task.BaseConfig{
							ID:   "process-{{ .item }}",
							Type: task.TaskTypeBasic,
						},
						BasicTask: task.BasicTask{
							Action: "process",
						},
					},
				},
			},
		}
		// Get task config
		taskConfig := &workflowConfig.Tasks[0]
		// Normalize the task
		err := orchestrator.NormalizeTask(t.Context(), workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)
		// Check collection config was normalized
		assert.Equal(t, "{{ .workflow.input.items }}", taskConfig.Items)
	})
}
func TestConfigOrchestrator_NormalizeAgentComponent(t *testing.T) {
	orchestrator := setupOrchestrator(t.Context(), t)
	t.Run("Should normalize agent with template expressions", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"model": "gpt-4",
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Setup task config
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"task_input": "value",
				},
			},
		}
		// Setup agent config
		agentConfig := &agent.Config{
			ID:           "agent1",
			Instructions: "Use model {{ .workflow.input.model }}",
			Model:        agent.Model{Config: core.ProviderConfig{}},
			With: &core.Input{
				"agent_input": "{{ .parent.with.task_input }}",
			},
		}
		// All task configs map
		allTaskConfigs := map[string]*task.Config{
			"task1": taskConfig,
		}
		// Normalize the agent
		err := orchestrator.NormalizeAgentComponent(
			t.Context(),
			workflowState,
			workflowConfig,
			taskConfig,
			agentConfig,
			allTaskConfigs,
		)
		require.NoError(t, err)
		// Check normalized values
		assert.Equal(t, "Use model gpt-4", agentConfig.Instructions)
		assert.Equal(t, "value", (*agentConfig.With)["agent_input"])
		// Check merged input
		assert.Equal(t, "value", (*agentConfig.With)["task_input"])
	})
}
func TestConfigOrchestrator_NormalizeToolComponent(t *testing.T) {
	orchestrator := setupOrchestrator(t.Context(), t)
	t.Run("Should normalize tool with template expressions", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"api_key": "secret123",
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Setup task config
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"endpoint": "https://api.example.com",
				},
			},
		}
		// Setup tool config
		toolConfig := &tool.Config{
			ID:          "tool1",
			Description: "API tool",
			With: &core.Input{
				"method": "POST",
				"url":    "{{ .parent.with.endpoint }}",
				"apiKey": "{{ .workflow.input.api_key }}",
			},
		}
		// All task configs map
		allTaskConfigs := map[string]*task.Config{
			"task1": taskConfig,
		}
		// Normalize the tool
		err := orchestrator.NormalizeToolComponent(
			t.Context(),
			workflowState,
			workflowConfig,
			taskConfig,
			toolConfig,
			allTaskConfigs,
		)
		require.NoError(t, err)
		// Check normalized values in With
		assert.Equal(t, "https://api.example.com", (*toolConfig.With)["url"])
		assert.Equal(t, "secret123", (*toolConfig.With)["apiKey"])
		// Check merged input
		assert.Equal(t, "https://api.example.com", (*toolConfig.With)["endpoint"])
		assert.Equal(t, "POST", (*toolConfig.With)["method"])
	})
}
func TestConfigOrchestrator_NormalizeTransitions(t *testing.T) {
	orchestrator := setupOrchestrator(t.Context(), t)
	t.Run("Should normalize success transition", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"next_task": "task2",
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Setup transition
		nextTask := "{{ .workflow.input.next_task }}"
		transition := &core.SuccessTransition{
			Next: &nextTask,
			With: &core.Input{
				"status": "completed",
			},
		}
		// All task configs map
		allTaskConfigs := map[string]*task.Config{}
		// Normalize the transition
		err := orchestrator.NormalizeSuccessTransition(
			t.Context(),
			transition,
			workflowState,
			workflowConfig,
			allTaskConfigs,
			nil,
		)
		require.NoError(t, err)
		// Check normalized values
		assert.Equal(t, "task2", *transition.Next)
	})
	t.Run("Should normalize error transition", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"error_handler": "handle-error",
			},
			Tasks: make(map[string]*task.State),
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Setup transition
		errorHandler := "{{ .workflow.input.error_handler }}"
		transition := &core.ErrorTransition{
			Next: &errorHandler,
			With: &core.Input{
				"error": "true",
			},
		}
		// All task configs map
		allTaskConfigs := map[string]*task.Config{}
		// Normalize the transition
		err := orchestrator.NormalizeErrorTransition(
			t.Context(),
			transition,
			workflowState,
			workflowConfig,
			allTaskConfigs,
			nil,
		)
		require.NoError(t, err)
		// Check normalized values
		assert.Equal(t, "handle-error", *transition.Next)
	})
}
func TestConfigOrchestrator_NormalizeOutputs(t *testing.T) {
	orchestrator := setupOrchestrator(t.Context(), t)
	t.Run("Should transform task output", func(t *testing.T) {
		// Setup workflow state
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Tasks: map[string]*task.State{
				"task1": {
					TaskExecID: "exec1",
					Output: &core.Output{
						"result": "success",
						"data":   map[string]any{"count": 10},
					},
				},
			},
		}
		// Setup workflow config
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Setup task config
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
		}
		// Task output
		taskOutput := &core.Output{
			"result": "success",
			"data":   map[string]any{"count": 10},
		}
		// Outputs config
		outputsConfig := &core.Input{
			"status":     "{{ .output.result }}",
			"item_count": "{{ .output.data.count }}",
		}
		// Transform the output
		transformedOutput, err := orchestrator.NormalizeTaskOutput(
			t.Context(),
			taskOutput,
			outputsConfig,
			workflowState,
			workflowConfig,
			taskConfig,
		)
		require.NoError(t, err)
		require.NotNil(t, transformedOutput)
		// Check transformed values
		assert.Equal(t, "success", (*transformedOutput)["status"])
		assert.Equal(t, 10, (*transformedOutput)["item_count"])
	})
}
