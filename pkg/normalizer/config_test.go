package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

func TestConfigNormalizer_NormalizeTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize task with workflow context and environment merging", func(t *testing.T) {
		taskConfig := &task.Config{
			ID:     "notification-task",
			Type:   task.TaskTypeBasic,
			Action: "notify_user",
			With: &core.Input{
				"recipient":        "{{ .workflow.input.email }}",
				"subject":          "Processing completed for {{ .workflow.input.request_id }}",
				"workflow_version": "{{ .workflow.version }}",
				"workflow_author":  "{{ .workflow.author.name }}",
			},
			Env: &core.EnvMap{
				"TASK_ENV": "task-value",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
			Input: &core.Input{
				"email":      "user@example.com",
				"request_id": "req-456",
			},
		}

		workflowConfig := &workflow.Config{
			ID:          "test-workflow",
			Version:     "1.2.0",
			Description: "Test workflow for notifications",
			Author: &core.Author{
				Name:  "John Doe",
				Email: "john@example.com",
			},
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"WORKFLOW_ENV": "workflow-value",
				},
			},
			Tasks: []task.Config{*taskConfig},
		}

		mergedEnv, err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)

		// Check template resolution
		assert.Equal(t, "user@example.com", (*taskConfig.With)["recipient"])
		assert.Equal(t, "Processing completed for req-456", (*taskConfig.With)["subject"])
		assert.Equal(t, "1.2.0", (*taskConfig.With)["workflow_version"])
		assert.Equal(t, "John Doe", (*taskConfig.With)["workflow_author"])

		// Check environment merging (workflow env should be merged with task env)
		assert.Equal(t, "workflow-value", mergedEnv["WORKFLOW_ENV"])
		assert.Equal(t, "task-value", mergedEnv["TASK_ENV"])
	})

	t.Run("Should normalize task referencing other tasks", func(t *testing.T) {
		analysisTask := &task.Config{
			ID:   "analysis-task",
			Type: task.TaskTypeBasic,
			With: &core.Input{
				"data":            "{{ .tasks.data_collector.output.dataset }}",
				"previous_action": "{{ .tasks.data_collector.action }}",
				"analyzer_type":   "{{ .tasks.config_loader.type }}",
				"threshold":       "{{ .tasks.config_loader.output.settings.threshold }}",
				"collector_final": "{{ .tasks.data_collector.final }}",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "analysis-workflow",
			WorkflowExecID: "exec-789",
			Tasks: map[string]*task.State{
				"data_collector": {
					Input: &core.Input{
						"source": "database",
					},
					Output: &core.Output{
						"dataset": "processed-data",
					},
				},
				"config_loader": {
					Input: &core.Input{
						"config_file": "settings.json",
					},
					Output: &core.Output{
						"settings": map[string]any{
							"threshold": "0.85",
						},
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "analysis-workflow",
			Tasks: []task.Config{
				{
					ID:     "data_collector",
					Type:   task.TaskTypeBasic,
					Action: "collect_data",
					Final:  true,
				},
				{
					ID:   "config_loader",
					Type: task.TaskTypeBasic,
				},
				*analysisTask,
			},
		}

		_, err := normalizer.NormalizeTask(workflowState, workflowConfig, analysisTask)
		require.NoError(t, err)

		// Check access to task outputs and config properties
		assert.Equal(t, "processed-data", (*analysisTask.With)["data"])
		assert.Equal(t, "collect_data", (*analysisTask.With)["previous_action"])
		assert.Equal(t, string(task.TaskTypeBasic), (*analysisTask.With)["analyzer_type"])
		assert.Equal(t, "0.85", (*analysisTask.With)["threshold"])
		assert.Equal(t, "true", (*analysisTask.With)["collector_final"])
	})

	t.Run("Should handle decision task condition normalization", func(t *testing.T) {
		decisionTask := &task.Config{
			ID:        "validation-task",
			Type:      task.TaskTypeDecision,
			Condition: `{{ eq .tasks.validator.output.status "valid" }}`,
		}

		workflowState := &workflow.State{
			WorkflowID:     "validation-workflow",
			WorkflowExecID: "exec-validation",
			Tasks: map[string]*task.State{
				"validator": {
					Output: &core.Output{
						"status": "valid",
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "validation-workflow",
			Tasks: []task.Config{
				{ID: "validator"},
				*decisionTask,
			},
		}

		_, err := normalizer.NormalizeTask(workflowState, workflowConfig, decisionTask)
		require.NoError(t, err)

		assert.Equal(t, "true", decisionTask.Condition)
	})
}

func TestConfigNormalizer_NormalizeAgentComponent(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize agent with complete parent context and environment merging", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "data-processor",
			Config: agent.ProviderConfig{
				Model: agent.ModelGPT4o,
			},
			Instructions: `You are processing data for the {{ .parent.id }} task.
The task type is {{ .parent.type }}.
Task action: {{ .parent.action }}
Task final: {{ .parent.final }}

Workflow context:
- Workflow ID: {{ .workflow.id }}
- Workflow Version: {{ .workflow.version }}

Data to process: {{ .tasks.data_fetcher.output.raw_data }}`,
			With: &core.Input{
				"context":    "{{ .tasks.context_builder.output.context }}",
				"task_type":  "{{ .parent.type }}",
				"task_final": "{{ .parent.final }}",
			},
			Env: &core.EnvMap{
				"AGENT_ENV": "agent-value",
			},
		}

		taskConfig := &task.Config{
			ID:     "processing-task",
			Type:   task.TaskTypeBasic,
			Action: "process_data",
			Final:  true,
			With: &core.Input{
				"city": "Seattle",
			},
			Env: &core.EnvMap{
				"TASK_ENV": "task-value",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "data-workflow",
			WorkflowExecID: "exec-data",
			Tasks: map[string]*task.State{
				"processing-task": {
					Input: &core.Input{
						"city": "Seattle",
					},
					Output: &core.Output{
						"status": "processing",
					},
				},
				"data_fetcher": {
					Output: &core.Output{
						"raw_data": "fetched-dataset",
					},
				},
				"context_builder": {
					Output: &core.Output{
						"context": "generated-context",
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID:      "data-workflow",
			Version: "2.1.0",
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"WORKFLOW_ENV": "workflow-value",
				},
			},
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		mergedEnv, err := normalizer.NormalizeAgentComponent(
			workflowState,
			workflowConfig,
			taskConfig,
			agentConfig,
			allTaskConfigs,
		)
		require.NoError(t, err)

		expectedInstructions := `You are processing data for the processing-task task.
The task type is basic.
Task action: process_data
Task final: true

Workflow context:
- Workflow ID: data-workflow
- Workflow Version: 2.1.0

Data to process: fetched-dataset`

		// Check template resolution with complete parent context
		assert.Equal(t, expectedInstructions, agentConfig.Instructions)
		assert.Equal(t, "generated-context", (*agentConfig.With)["context"])
		assert.Equal(t, string(task.TaskTypeBasic), (*agentConfig.With)["task_type"])
		assert.Equal(t, "true", (*agentConfig.With)["task_final"])

		// Check environment merging (workflow -> task -> agent)
		assert.Equal(t, "workflow-value", mergedEnv["WORKFLOW_ENV"])
		assert.Equal(t, "task-value", mergedEnv["TASK_ENV"])
		assert.Equal(t, "agent-value", mergedEnv["AGENT_ENV"])
	})

	t.Run("Should normalize agent actions with parent agent context", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "analyzer-agent",
			Config: agent.ProviderConfig{
				Model: agent.ModelGPT4oMini,
			},
			Instructions: "Analyze data for tasks",
			With: &core.Input{
				"data":      "test-data",
				"threshold": "0.95",
			},
			Actions: []*agent.ActionConfig{
				{
					ID: "analyze-data",
					Prompt: `Analyze the following data:

Agent context:
- Agent ID: {{ .parent.id }}
- Agent Instructions: {{ .parent.instructions }}
- Agent Model: {{ .parent.config.Model }}

Task context:
- City to analyze: {{ .parent.input.data }}

Previous results: {{ .tasks.preprocessing.output.summary }}`,
					With: &core.Input{
						"context":   "{{ .parent.input.data }}",
						"threshold": "{{ .parent.input.threshold | default \"0.8\" }}",
					},
				},
			},
		}

		taskConfig := &task.Config{
			ID:     "analysis-task",
			Action: "analyze-data",
		}

		workflowState := &workflow.State{
			WorkflowID:     "analysis-workflow",
			WorkflowExecID: "exec-analysis",
			Tasks: map[string]*task.State{
				"preprocessing": {
					Output: &core.Output{
						"summary": "preprocessing-complete",
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID:    "analysis-workflow",
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		_, err := normalizer.NormalizeAgentComponent(
			workflowState,
			workflowConfig,
			taskConfig,
			agentConfig,
			allTaskConfigs,
		)
		require.NoError(t, err)

		action := agentConfig.Actions[0]
		expectedPrompt := `Analyze the following data:

Agent context:
- Agent ID: analyzer-agent
- Agent Instructions: Analyze data for tasks
- Agent Model: gpt-4o-mini

Task context:
- City to analyze: test-data

Previous results: preprocessing-complete`

		assert.Equal(t, expectedPrompt, action.Prompt)
		assert.Equal(t, "test-data", (*action.With)["context"])
		assert.Equal(t, "0.95", (*action.With)["threshold"])
	})
}

func TestConfigNormalizer_NormalizeToolComponent(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize tool with complete parent context and environment merging", func(t *testing.T) {
		toolConfig := &tool.Config{
			ID:          "api-caller",
			Execute:     "{{ .env.SCRIPTS_PATH }}/api_call.ts",
			Description: "API caller for {{ .parent.id }} task of type {{ .parent.type }}",
			With: &core.Input{
				"endpoint_path": "users",
				"task_action":   "{{ .parent.action }}",
				"task_final":    "{{ .parent.final }}",
				"workflow_id":   "{{ .workflow.id }}",
				"headers": map[string]any{
					"authorization": "Bearer {{ .parent.input.token }}",
					"x-workflow-id": "{{ .workflow.id }}",
					"x-task-id":     "{{ .parent.id }}",
				},
			},
			Env: &core.EnvMap{
				"TOOL_ENV": "tool-value",
			},
		}

		taskConfig := &task.Config{
			ID:     "api-task",
			Type:   task.TaskTypeBasic,
			Action: "fetch_data",
			Final:  false,
			With: &core.Input{
				"token": "secret-token",
			},
			Env: &core.EnvMap{
				"TASK_ENV": "task-value",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "api-workflow",
			WorkflowExecID: "exec-api",
			Tasks: map[string]*task.State{
				"api-task": {
					Input: &core.Input{
						"token": "secret-token",
					},
					Output: &core.Output{
						"status": "pending",
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "api-workflow",
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"WORKFLOW_ENV": "workflow-value",
					"SCRIPTS_PATH": "/app/scripts",
					"API_BASE_URL": "https://api.example.com",
				},
			},
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		mergedEnv, err := normalizer.NormalizeToolComponent(
			workflowState,
			workflowConfig,
			taskConfig,
			toolConfig,
			allTaskConfigs,
		)
		require.NoError(t, err)

		// Check template resolution with complete parent context
		assert.Equal(t, "/app/scripts/api_call.ts", toolConfig.Execute)
		assert.Equal(t, "API caller for api-task task of type basic", toolConfig.Description)
		assert.Equal(t, "users", (*toolConfig.With)["endpoint_path"]) // This is not templated
		assert.Equal(t, "fetch_data", (*toolConfig.With)["task_action"])
		assert.Equal(t, "false", (*toolConfig.With)["task_final"])
		assert.Equal(t, "api-workflow", (*toolConfig.With)["workflow_id"])

		// Check nested object resolution
		headers := (*toolConfig.With)["headers"].(map[string]any)
		assert.Equal(t, "Bearer secret-token", headers["authorization"])
		assert.Equal(t, "api-workflow", headers["x-workflow-id"])
		assert.Equal(t, "api-task", headers["x-task-id"])

		// Check environment merging (workflow -> task -> tool)
		assert.Equal(t, "workflow-value", mergedEnv["WORKFLOW_ENV"])
		assert.Equal(t, "/app/scripts", mergedEnv["SCRIPTS_PATH"])
		assert.Equal(t, "https://api.example.com", mergedEnv["API_BASE_URL"])
		assert.Equal(t, "task-value", mergedEnv["TASK_ENV"])
		assert.Equal(t, "tool-value", mergedEnv["TOOL_ENV"])
	})
}

func TestConfigNormalizer_TaskCallingSubTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should support task calling another task with parent context", func(t *testing.T) {
		// This simulates the case where a parent task calls a subtask
		// The subtask should have access to parent task properties
		subtaskConfig := &task.Config{
			ID:     "process-item",
			Type:   task.TaskTypeBasic,
			Action: "process",
			With: &core.Input{
				"item":          "{{ .parent.input.current_item }}",
				"batch_id":      "{{ .parent.id }}",
				"parent_action": "{{ .parent.action }}",
				"parent_type":   "{{ .parent.type }}",
				"workflow_id":   "{{ .workflow.id }}",
				"config":        "{{ .tasks.config_loader.output.settings }}",
			},
			Env: &core.EnvMap{
				"PARENT_TASK": "{{ .parent.id }}",
				"PARENT_TYPE": "{{ .parent.type }}",
			},
		}

		parentTaskConfig := &task.Config{
			ID:     "batch-processor",
			Type:   task.TaskTypeBasic,
			Action: "batch_process",
			With: &core.Input{
				"current_item": "item-123",
				"batch_size":   "10",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "batch-workflow",
			WorkflowExecID: "exec-batch",
			Tasks: map[string]*task.State{
				"batch-processor": {
					Input: &core.Input{
						"current_item": "item-123",
						"batch_size":   "10",
					},
					Output: &core.Output{
						"status": "processing",
					},
				},
				"config_loader": {
					Output: &core.Output{
						"settings": map[string]any{
							"timeout": "30s",
						},
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "batch-workflow",
			Tasks: []task.Config{
				*parentTaskConfig,
				{ID: "config_loader"},
				*subtaskConfig,
			},
		}

		// Create context with parent task config to simulate task-to-task relationship
		allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
		normCtx := &NormalizationContext{
			WorkflowState:    workflowState,
			WorkflowConfig:   workflowConfig,
			TaskConfigs:      allTaskConfigsMap,
			ParentTaskConfig: parentTaskConfig, // This indicates parent is a task
			MergedEnv:        core.EnvMap{},
		}

		err := normalizer.normalizer.NormalizeTaskConfig(subtaskConfig, normCtx)
		require.NoError(t, err)

		// Verify access to parent task properties and runtime state
		assert.Equal(t, "item-123", (*subtaskConfig.With)["item"])
		assert.Equal(t, "batch-processor", (*subtaskConfig.With)["batch_id"])
		assert.Equal(t, "batch_process", (*subtaskConfig.With)["parent_action"])
		assert.Equal(t, string(task.TaskTypeBasic), (*subtaskConfig.With)["parent_type"])
		assert.Equal(t, "batch-workflow", (*subtaskConfig.With)["workflow_id"])

		// Verify access to sibling task outputs
		settings := (*subtaskConfig.With)["config"].(map[string]any)
		assert.Equal(t, "30s", settings["timeout"])

		// Verify environment template resolution
		assert.Equal(t, "batch-processor", subtaskConfig.GetEnv().Prop("PARENT_TASK"))
		assert.Equal(t, string(task.TaskTypeBasic), subtaskConfig.GetEnv().Prop("PARENT_TYPE"))
	})
}

func TestConfigNormalizer_ErrorHandling(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should return error for missing template key in environment", func(t *testing.T) {
		taskConfig := &task.Config{
			ID: "error-task",
			Env: &core.EnvMap{
				"INVALID": "{{ .invalid.template }}",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: "exec-error",
		}

		workflowConfig := &workflow.Config{
			ID:    "error-workflow",
			Tasks: []task.Config{*taskConfig},
		}

		_, err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Should return error for missing template key in agent normalization", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID:           "error-agent",
			Instructions: "{{ .invalid.template }}",
		}

		taskConfig := &task.Config{
			ID: "parent-task",
		}

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: "exec-error",
		}

		workflowConfig := &workflow.Config{
			ID:    "error-workflow",
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		_, err := normalizer.NormalizeAgentComponent(
			workflowState,
			workflowConfig,
			taskConfig,
			agentConfig,
			allTaskConfigs,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Should return error for missing template key in tool normalization", func(t *testing.T) {
		toolConfig := &tool.Config{
			ID:      "error-tool",
			Execute: "{{ .invalid.template }}",
		}

		taskConfig := &task.Config{
			ID: "parent-task",
		}

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: "exec-error",
		}

		workflowConfig := &workflow.Config{
			ID:    "error-workflow",
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		_, err := normalizer.NormalizeToolComponent(
			workflowState,
			workflowConfig,
			taskConfig,
			toolConfig,
			allTaskConfigs,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}

func TestConfigNormalizer_BuildTaskConfigsMap(t *testing.T) {
	t.Run("Should convert task config slice to map", func(t *testing.T) {
		taskConfigs := []task.Config{
			{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
			{
				ID:   "task2",
				Type: task.TaskTypeDecision,
			},
		}

		configMap := BuildTaskConfigsMap(taskConfigs)

		assert.Len(t, configMap, 2)
		assert.Equal(t, "task1", configMap["task1"].ID)
		assert.Equal(t, task.TaskTypeBasic, configMap["task1"].Type)
		assert.Equal(t, "task2", configMap["task2"].ID)
		assert.Equal(t, task.TaskTypeDecision, configMap["task2"].Type)
	})

	t.Run("Should handle empty task config slice", func(t *testing.T) {
		taskConfigs := []task.Config{}
		configMap := BuildTaskConfigsMap(taskConfigs)
		assert.Empty(t, configMap)
	})
}
