package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

func TestConfigNormalizer_NormalizeTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize task with workflow context and environment merging", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "notification-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"recipient":        "{{ .workflow.input.email }}",
					"subject":          "Processing completed for {{ .workflow.input.request_id }}",
					"workflow_version": "{{ .workflow.version }}",
					"workflow_author":  "{{ .workflow.author.name }}",
				},
				Env: &core.EnvMap{
					"TASK_ENV": "task-value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "notify_user",
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

		err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)

		// Check template resolution
		assert.Equal(t, "user@example.com", (*taskConfig.With)["recipient"])
		assert.Equal(t, "Processing completed for req-456", (*taskConfig.With)["subject"])
		assert.Equal(t, "1.2.0", (*taskConfig.With)["workflow_version"])
		assert.Equal(t, "John Doe", (*taskConfig.With)["workflow_author"])

		// Check environment merging (workflow env should be merged with task env)
		assert.Equal(t, "workflow-value", taskConfig.GetEnv().Prop("WORKFLOW_ENV"))
		assert.Equal(t, "task-value", taskConfig.GetEnv().Prop("TASK_ENV"))
	})

	t.Run("Should normalize task referencing other tasks", func(t *testing.T) {
		analysisTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "analysis-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"data":            "{{ .tasks.data_collector.output.dataset }}",
					"previous_action": "{{ .tasks.data_collector.action }}",
					"analyzer_type":   "{{ .tasks.config_loader.type }}",
					"threshold":       "{{ .tasks.config_loader.output.settings.threshold }}",
					"collector_final": "{{ .tasks.data_collector.final }}",
				},
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
					BaseConfig: task.BaseConfig{
						ID:    "data_collector",
						Type:  task.TaskTypeBasic,
						Final: true,
					},
					BasicTask: task.BasicTask{
						Action: "collect_data",
					},
				},
				{
					BaseConfig: task.BaseConfig{
						ID:    "config_loader",
						Type:  task.TaskTypeBasic,
						Final: true,
					},
				},
				*analysisTask,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, analysisTask)
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
			BaseConfig: task.BaseConfig{
				ID:   "validation-task",
				Type: task.TaskTypeDecision,
			},
			DecisionTask: task.DecisionTask{
				Condition: `{{ eq .tasks.validator.output.status "valid" }}`,
			},
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
				{
					BaseConfig: task.BaseConfig{
						ID:   "validator",
						Type: task.TaskTypeBasic,
					},
				},
				*decisionTask,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, decisionTask)
		require.NoError(t, err)

		assert.Equal(t, "true", decisionTask.Condition)
	})
}

func TestConfigNormalizer_NormalizeAgentComponent(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize agent with complete parent context and environment merging", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "data-processor",
			Config: core.ProviderConfig{
				Model: "gpt-4o",
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
			BaseConfig: task.BaseConfig{
				ID:    "processing-task",
				Type:  task.TaskTypeBasic,
				Final: true,
				With: &core.Input{
					"city": "Seattle",
				},
				Env: &core.EnvMap{
					"TASK_ENV": "task-value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process_data",
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

		err := normalizer.NormalizeAgentComponent(
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
		assert.Equal(t, "workflow-value", agentConfig.GetEnv().Prop("WORKFLOW_ENV"))
		assert.Equal(t, "task-value", agentConfig.GetEnv().Prop("TASK_ENV"))
		assert.Equal(t, "agent-value", agentConfig.GetEnv().Prop("AGENT_ENV"))
	})

	t.Run("Should normalize agent actions with parent agent context", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "analyzer-agent",
			Config: core.ProviderConfig{
				Model: "gpt-4o-mini",
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
			BaseConfig: task.BaseConfig{
				ID:   "analysis-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "analyze-data",
			},
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

		err := normalizer.NormalizeAgentComponent(
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
			BaseConfig: task.BaseConfig{
				ID:    "api-task",
				Type:  task.TaskTypeBasic,
				Final: false,
				With: &core.Input{
					"token": "secret-token",
				},
				Env: &core.EnvMap{
					"TASK_ENV": "task-value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "fetch_data",
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

		err := normalizer.NormalizeToolComponent(
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
		assert.Equal(t, "workflow-value", toolConfig.GetEnv().Prop("WORKFLOW_ENV"))
		assert.Equal(t, "/app/scripts", toolConfig.GetEnv().Prop("SCRIPTS_PATH"))
		assert.Equal(t, "https://api.example.com", toolConfig.GetEnv().Prop("API_BASE_URL"))
		assert.Equal(t, "task-value", toolConfig.GetEnv().Prop("TASK_ENV"))
		assert.Equal(t, "tool-value", toolConfig.GetEnv().Prop("TOOL_ENV"))
	})
}

func TestConfigNormalizer_TaskCallingSubTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should support task calling another task with parent context", func(t *testing.T) {
		// This simulates the case where a parent task calls a subtask
		// The subtask should have access to parent task properties
		subtaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "process-item",
				Type: task.TaskTypeBasic,
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
			},
			BasicTask: task.BasicTask{
				Action: "process",
			},
		}

		parentTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "batch-processor",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"current_item": "item-123",
					"batch_size":   "10",
				},
			},
			BasicTask: task.BasicTask{
				Action: "batch_process",
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
				{
					BaseConfig: task.BaseConfig{
						ID:   "config_loader",
						Type: task.TaskTypeBasic,
					},
				},
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
			MergedEnv:        &core.EnvMap{},
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
			BaseConfig: task.BaseConfig{
				ID: "error-task",
				Env: &core.EnvMap{
					"INVALID": "{{ .invalid.template }}",
				},
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

		err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})

	t.Run("Should return error for missing template key in agent normalization", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID:           "error-agent",
			Instructions: "{{ .invalid.template }}",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "parent-task",
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

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		err := normalizer.NormalizeAgentComponent(
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
			BaseConfig: task.BaseConfig{
				ID: "parent-task",
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

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		err := normalizer.NormalizeToolComponent(
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
				BaseConfig: task.BaseConfig{
					ID:   "task1",
					Type: task.TaskTypeBasic,
				},
			},
			{
				BaseConfig: task.BaseConfig{
					ID:   "task2",
					Type: task.TaskTypeDecision,
				},
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

func TestConfigNormalizer_ProviderConfigNormalization(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize ProviderConfig templates including APIKey", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "test-agent",
			Config: core.ProviderConfig{
				Provider: core.ProviderGroq,
				Model:    "llama-3.3-70b-versatile",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				APIURL:   "{{ .env.BASE_URL }}/v1",
			},
			Instructions: "Test instructions",
			With: &core.Input{
				"test": "value",
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
			BasicTask: task.BasicTask{
				Action: "test",
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
		}

		workflowConfig := &workflow.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{*taskConfig},
		}

		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)

		// Create normalization context with environment containing the API key
		normCtx := &NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfigs:    allTaskConfigs,
			ParentConfig: map[string]any{
				"id": "test-task",
			},
			CurrentInput: agentConfig.With,
			MergedEnv: &core.EnvMap{
				"OPENAI_API_KEY": "sk-test-api-key-12345",
				"BASE_URL":       "https://api.test.com",
			},
		}

		err := normalizer.normalizer.NormalizeAgentConfig(agentConfig, normCtx, "")
		require.NoError(t, err)

		// The API key template should be resolved
		assert.Equal(t, "sk-test-api-key-12345", agentConfig.Config.APIKey)
		assert.Equal(t, "https://api.test.com/v1", agentConfig.Config.APIURL)
	})
}

func TestConfigNormalizer_MapstructureCompatibility(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should handle all mapstructure field mappings correctly", func(t *testing.T) {
		// Test task config with on_success, on_error, and config fields
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				Config: core.GlobalOpts{
					ScheduleToStartTimeout: "{{ .env.SCHEDULE_TIMEOUT }}",
					StartToCloseTimeout:    "{{ .env.START_TIMEOUT }}",
					RetryPolicy: &core.RetryPolicyConfig{
						InitialInterval: "{{ .env.INITIAL_INTERVAL }}",
						MaximumAttempts: 5,
					},
				},
				OnSuccess: &core.SuccessTransition{
					Next: &[]string{"next-task"}[0],
					With: &core.Input{
						"message": "{{ .env.SUCCESS_MSG }}",
					},
				},
				OnError: &core.ErrorTransition{
					Next: &[]string{"error-handler"}[0],
					With: &core.Input{
						"error": "{{ .env.ERROR_MSG }}",
					},
				},
			},
		}

		// Test agent config with config field (ProviderConfig)
		agentConfig := &agent.Config{
			ID: "test-agent",
			Config: core.ProviderConfig{
				Provider: core.ProviderOpenAI,
				Model:    "gpt-4o",
				APIKey:   "{{ .env.API_KEY }}",
				APIURL:   "{{ .env.API_URL }}",
			},
			Instructions: "Test agent with API key {{ .env.API_KEY }}",
		}

		// Test workflow config with config field (Opts)
		workflowConfig := &workflow.Config{
			ID:      "test-workflow",
			Version: "{{ .env.VERSION }}",
			Author: &core.Author{
				Name:  "{{ .env.AUTHOR_NAME }}",
				Email: "{{ .env.AUTHOR_EMAIL }}",
			},
			Opts: workflow.Opts{
				GlobalOpts: core.GlobalOpts{
					ScheduleToStartTimeout: "{{ .env.WF_TIMEOUT }}",
				},
				Env: &core.EnvMap{
					"WF_VAR": "{{ .env.WORKFLOW_VAR }}",
				},
			},
			Tasks:  []task.Config{*taskConfig},
			Agents: []agent.Config{*agentConfig},
		}

		// Test project config with config field (Opts)
		projectConfig := &project.Config{
			Name:        "{{ .env.PROJECT_NAME }}",
			Version:     "{{ .env.PROJECT_VERSION }}",
			Description: "Test project with version {{ .env.PROJECT_VERSION }}",
			Author: core.Author{
				Name:  "{{ .env.PROJECT_AUTHOR }}",
				Email: "{{ .env.PROJECT_EMAIL }}",
			},
			Opts: project.Opts{
				GlobalOpts: core.GlobalOpts{
					StartToCloseTimeout: "{{ .env.PROJECT_TIMEOUT }}",
				},
			},
			Workflows: []*project.WorkflowSourceConfig{
				{
					Source: "{{ .env.WORKFLOW_SOURCE }}",
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
		}

		// Environment with values for all the templates
		testEnv := core.EnvMap{
			"SCHEDULE_TIMEOUT": "2m",
			"START_TIMEOUT":    "5m",
			"INITIAL_INTERVAL": "1s",
			"SUCCESS_MSG":      "Task completed successfully",
			"ERROR_MSG":        "Task failed",
			"API_KEY":          "sk-test-key-123",
			"API_URL":          "https://api.openai.com/v1",
			"VERSION":          "1.2.3",
			"AUTHOR_NAME":      "John Doe",
			"AUTHOR_EMAIL":     "john@example.com",
			"WF_TIMEOUT":       "10m",
			"WORKFLOW_VAR":     "workflow-value",
			"PROJECT_NAME":     "My Project",
			"PROJECT_VERSION":  "2.0.0",
			"PROJECT_AUTHOR":   "Jane Smith",
			"PROJECT_EMAIL":    "jane@example.com",
			"PROJECT_TIMEOUT":  "15m",
			"WORKFLOW_SOURCE":  "./workflows/main.yaml",
		}

		// Test task normalization (on_success, on_error, config fields)
		allTaskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)
		normCtx := &NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfigs:    allTaskConfigs,
			MergedEnv:      &testEnv,
		}

		err := normalizer.normalizer.NormalizeTaskConfig(taskConfig, normCtx)
		require.NoError(t, err)

		// Verify task config templates were resolved (including nested structures)
		assert.Equal(t, "2m", taskConfig.Config.ScheduleToStartTimeout)
		assert.Equal(t, "5m", taskConfig.Config.StartToCloseTimeout)
		assert.Equal(t, "1s", taskConfig.Config.RetryPolicy.InitialInterval)
		assert.Equal(t, int32(5), taskConfig.Config.RetryPolicy.MaximumAttempts)
		assert.Equal(t, "Task completed successfully", (*taskConfig.OnSuccess.With)["message"])
		assert.Equal(t, "Task failed", (*taskConfig.OnError.With)["error"])

		// Test agent normalization (config field with ProviderConfig)
		err = normalizer.normalizer.NormalizeAgentConfig(agentConfig, normCtx, "")
		require.NoError(t, err)

		// Verify agent config templates were resolved
		assert.Equal(t, "sk-test-key-123", agentConfig.Config.APIKey)
		assert.Equal(t, "https://api.openai.com/v1", agentConfig.Config.APIURL)
		assert.Equal(t, "Test agent with API key sk-test-key-123", agentConfig.Instructions)

		// Test direct config serialization/deserialization
		// This tests the FromMapDefault path directly
		configMap, err := projectConfig.AsMap()
		require.NoError(t, err)

		// Parse templates in the map
		context := map[string]any{"env": testEnv}
		parsed, err := normalizer.normalizer.engine.ParseMap(configMap, context)
		require.NoError(t, err)

		// Deserialize back to struct (this is where mapstructure tags are crucial)
		newProjectConfig := &project.Config{}
		err = newProjectConfig.FromMap(parsed)
		require.NoError(t, err)

		// Verify project config templates were resolved correctly
		assert.Equal(t, "My Project", newProjectConfig.Name)
		assert.Equal(t, "2.0.0", newProjectConfig.Version)
		assert.Equal(t, "Test project with version 2.0.0", newProjectConfig.Description)
		assert.Equal(t, "Jane Smith", newProjectConfig.Author.Name)
		assert.Equal(t, "jane@example.com", newProjectConfig.Author.Email)
		assert.Equal(t, "15m", newProjectConfig.Opts.StartToCloseTimeout)
		assert.Equal(t, "./workflows/main.yaml", newProjectConfig.Workflows[0].Source)
	})
}

func TestConfigNormalizer_NormalizeParallelTask(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize parallel task with sub-tasks containing templates", func(t *testing.T) {
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "process_data_parallel",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"raw_data": "sample data",
					"content":  "This is a great product! I love it.",
				},
				Env: &core.EnvMap{
					"PARALLEL_TIMEOUT": "5m",
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 4,
				Timeout:    "5m",
				Tasks: []task.Config{
					{
						BaseConfig: task.BaseConfig{
							ID:   "sentiment_analysis",
							Type: task.TaskTypeBasic,
							With: &core.Input{
								"text": "{{ .workflow.input.content }}",
							},
						},
						BasicTask: task.BasicTask{
							Action: "analyze_sentiment",
						},
					},
					{
						BaseConfig: task.BaseConfig{
							ID:   "extract_keywords",
							Type: task.TaskTypeBasic,
							With: &core.Input{
								"text":         "{{ .workflow.input.content }}",
								"max_keywords": 10,
							},
						},
						BasicTask: task.BasicTask{
							Action: "extract",
						},
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
			Input: &core.Input{
				"content":  "This is great!",
				"raw_data": "sample",
			},
		}

		workflowConfig := &workflow.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{*parallelTaskConfig},
		}

		// Check what templates look like before normalization
		t.Logf("Before normalization - subTask1 text: %v", (*parallelTaskConfig.Tasks[0].With)["text"])
		t.Logf("Before normalization - subTask2 text: %v", (*parallelTaskConfig.Tasks[1].With)["text"])

		// Normalize the parallel task
		err := normalizer.NormalizeTask(workflowState, workflowConfig, parallelTaskConfig)
		require.NoError(t, err)

		// Check what templates look like after normalization
		t.Logf("After normalization - subTask1 text: %v", (*parallelTaskConfig.Tasks[0].With)["text"])
		t.Logf("After normalization - subTask2 text: %v", (*parallelTaskConfig.Tasks[1].With)["text"])

		// Check that sub-task templates were resolved
		subTask1 := parallelTaskConfig.Tasks[0]
		assert.Equal(t, "This is great!", (*subTask1.With)["text"])

		subTask2 := parallelTaskConfig.Tasks[1]
		assert.Equal(t, "This is great!", (*subTask2.With)["text"])
		// Fix type assertion for max_keywords - it might be converted to float64 by JSON unmarshaling
		maxKeywords := (*subTask2.With)["max_keywords"]
		switch v := maxKeywords.(type) {
		case int:
			assert.Equal(t, 10, v)
		case float64:
			assert.Equal(t, float64(10), v)
		default:
			t.Errorf("max_keywords has unexpected type: %T", maxKeywords)
		}
	})

	t.Run("Should handle sub-task normalization that references parent parallel task context", func(t *testing.T) {
		// Test what happens when we try to normalize individual sub-tasks
		// that might need parent parallel task context
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "batch_processor",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"batch_id":   "batch-123",
					"batch_size": 10,
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy: task.StrategyWaitAll,
				Tasks: []task.Config{
					{
						BaseConfig: task.BaseConfig{
							ID:   "process_item_1",
							Type: task.TaskTypeBasic,
							With: &core.Input{
								"item_id":   "item-1",
								"parent_id": "{{ .parent.id }}",             // Should reference parallel task
								"batch_id":  "{{ .parent.input.batch_id }}", // Should reference parent input
							},
						},
						BasicTask: task.BasicTask{
							Action: "process",
						},
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
		}

		workflowConfig := &workflow.Config{
			ID:    "test-workflow",
			Tasks: []task.Config{*parallelTaskConfig},
		}

		// This tests the current behavior - parallel task normalization should work
		err := normalizer.NormalizeTask(workflowState, workflowConfig, parallelTaskConfig)
		require.NoError(t, err)

		// Check that basic fields are handled correctly
		subTask := parallelTaskConfig.Tasks[0]
		assert.Equal(t, "item-1", (*subTask.With)["item_id"])

		// These templates should remain unresolved since there's no parent context
		// established for sub-tasks yet (this is what we need to potentially fix)
		t.Logf("parent_id after normalization: %v", (*subTask.With)["parent_id"])
		t.Logf("batch_id after normalization: %v", (*subTask.With)["batch_id"])
	})

	t.Run("Should support nested output access for parallel tasks", func(t *testing.T) {
		// Test task that references nested parallel task outputs
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "aggregator_task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					// Access nested sub-task outputs using tasks.parallel_task.output.subtask_id.output format
					"sentiment_result": "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.sentiment }}",
					"keywords_result":  "{{ .tasks.process_data_parallel.output.extract_keywords.output.keywords }}",
					"analysis_score":   "{{ .tasks.process_data_parallel.output.sentiment_analysis.output.confidence }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate",
			},
		}

		// Mock parallel task state with aggregated sub-task outputs
		parallelState := &task.ParallelState{
			SubTasks: map[string]*task.State{
				"sentiment_analysis": {
					Output: &core.Output{
						"sentiment":  "positive",
						"confidence": 0.95,
					},
				},
				"extract_keywords": {
					Output: &core.Output{
						"keywords": []string{"great", "product", "love"},
						"count":    3,
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: "exec-123",
			Tasks: map[string]*task.State{
				"process_data_parallel": {
					TaskID:        "process_data_parallel",
					ExecutionType: task.ExecutionParallel,
					ParallelState: parallelState,
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "process_data_parallel",
						Type: task.TaskTypeParallel,
					},
				},
				*taskConfig,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, taskConfig)
		require.NoError(t, err)

		// Verify nested output access works
		assert.Equal(t, "positive", (*taskConfig.With)["sentiment_result"])
		assert.Equal(t, []string{"great", "product", "love"}, (*taskConfig.With)["keywords_result"])
		assert.Equal(t, "0.95", (*taskConfig.With)["analysis_score"])
	})

	t.Run("Should demonstrate complete parallel task workflow with nested access", func(t *testing.T) {
		// This test demonstrates a complete workflow:
		// 1. A parallel task that processes data in sub-tasks
		// 2. An aggregator task that accesses the nested outputs
		// 3. A final task that uses the aggregated results

		// Define the parallel task configuration (would contain sub-tasks)
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel_processor",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy: task.StrategyWaitAll,
				Tasks: []task.Config{
					{
						BaseConfig: task.BaseConfig{
							ID:   "sentiment_analysis",
							Type: task.TaskTypeBasic,
						},
					},
					{
						BaseConfig: task.BaseConfig{
							ID:   "keyword_extraction",
							Type: task.TaskTypeBasic,
						},
					},
					{
						BaseConfig: task.BaseConfig{
							ID:   "performance_monitor",
							Type: task.TaskTypeBasic,
						},
					},
				},
			},
		}

		// Define an aggregator task that uses nested output access
		aggregatorConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "aggregate_results",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"sentiment":   "{{ .tasks.parallel_processor.output.sentiment_analysis.output.sentiment }}",
					"keywords":    "{{ .tasks.parallel_processor.output.keyword_extraction.output.keywords }}",
					"confidence":  "{{ .tasks.parallel_processor.output.sentiment_analysis.output.confidence }}",
					"duration":    "{{ .tasks.parallel_processor.output.performance_monitor.output.duration }}",
					"full_result": "{{ .tasks.parallel_processor.output.sentiment_analysis.output }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate",
			},
		}

		// Define a final task that uses the aggregated results
		finalTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "generate_report",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"aggregated_data": "{{ .tasks.aggregate_results.output.summary }}",
					"total_keywords":  "{{ len .tasks.parallel_processor.output.keyword_extraction.output.keywords }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "generate_report",
			},
		}

		// Mock the workflow state with parallel task outputs
		parallelState := &task.ParallelState{
			SubTasks: map[string]*task.State{
				"sentiment_analysis": {
					Output: &core.Output{
						"sentiment":  "positive",
						"confidence": 0.92,
						"details":    "High confidence positive sentiment detected",
					},
				},
				"keyword_extraction": {
					Output: &core.Output{
						"keywords": []string{"excellent", "quality", "recommend", "satisfied"},
						"count":    4,
					},
				},
				"performance_monitor": {
					Output: &core.Output{
						"duration":    "2.3s",
						"memory_used": "45MB",
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "analysis-workflow",
			WorkflowExecID: "exec-analysis",
			Tasks: map[string]*task.State{
				"parallel_processor": {
					TaskID:        "parallel_processor",
					ExecutionType: task.ExecutionParallel,
					ParallelState: parallelState,
				},
				"aggregate_results": {
					TaskID: "aggregate_results",
					Output: &core.Output{
						"summary": map[string]any{
							"sentiment":       "positive",
							"keyword_count":   4,
							"confidence":      0.92,
							"processing_time": "2.3s",
						},
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "analysis-workflow",
			Tasks: []task.Config{
				*parallelTaskConfig,
				*aggregatorConfig,
				*finalTaskConfig,
			},
		}

		// Test normalization of the aggregator task (accessing nested outputs)
		err := normalizer.NormalizeTask(workflowState, workflowConfig, aggregatorConfig)
		require.NoError(t, err)

		// Verify the aggregator task can access nested outputs
		assert.Equal(t, "positive", (*aggregatorConfig.With)["sentiment"])
		assert.Equal(
			t,
			[]string{"excellent", "quality", "recommend", "satisfied"},
			(*aggregatorConfig.With)["keywords"],
		)
		assert.Equal(t, "0.92", (*aggregatorConfig.With)["confidence"])
		assert.Equal(t, "2.3s", (*aggregatorConfig.With)["duration"])

		// Verify it can access the full output object
		fullResult := (*aggregatorConfig.With)["full_result"].(map[string]any)
		assert.Equal(t, "positive", fullResult["sentiment"])
		assert.Equal(t, "High confidence positive sentiment detected", fullResult["details"])

		// Test normalization of the final task (accessing both nested and regular outputs)
		err = normalizer.NormalizeTask(workflowState, workflowConfig, finalTaskConfig)
		require.NoError(t, err)

		// Verify the final task can access both aggregated and nested outputs
		expectedSummary := map[string]any{
			"sentiment":       "positive",
			"keyword_count":   4, // Keep original type from the mock data
			"confidence":      0.92,
			"processing_time": "2.3s",
		}
		assert.Equal(t, expectedSummary, (*finalTaskConfig.With)["aggregated_data"])
		assert.Equal(t, "4", (*finalTaskConfig.With)["total_keywords"]) // len() function results are now strings
	})
}

func TestConfigNormalizer_NestedParallelTasks(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize two levels of nested parallel tasks with templates", func(t *testing.T) {
		// Create a deeply nested structure:
		// batch_processor (parallel)
		//   ├── data_processor (parallel)
		//   │   ├── sentiment_analysis (basic)
		//   │   └── keyword_extraction (basic)
		//   └── metadata_processor (basic)

		nestedParallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "batch_processor",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"batch_id":   "batch-456",
					"batch_size": 100,
					"priority":   "high",
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 2,
				Tasks: []task.Config{
					// First sub-task: another parallel task
					{
						BaseConfig: task.BaseConfig{
							ID:   "data_processor",
							Type: task.TaskTypeParallel,
							With: &core.Input{
								"processor_id":    "dp-{{ .parent.input.batch_id }}",
								"parent_batch":    "{{ .parent.id }}",
								"parent_priority": "{{ .parent.input.priority }}",
								"workflow_id":     "{{ .workflow.id }}",
							},
						},
						ParallelTask: task.ParallelTask{
							Strategy: task.StrategyWaitAll,
							Tasks: []task.Config{
								// Deeply nested basic task 1
								{
									BaseConfig: task.BaseConfig{
										ID:   "sentiment_analysis",
										Type: task.TaskTypeBasic,
										With: &core.Input{
											"text":              "{{ .workflow.input.content }}",
											"processor_parent":  "{{ .parent.id }}",
											"batch_parent":      "{{ .parent.input.parent_batch }}",
											"original_priority": "{{ .parent.input.parent_priority }}",
											"task_chain":        "{{ .workflow.id }}.{{ .parent.input.parent_batch }}.{{ .parent.id }}.sentiment_analysis",
										},
									},
									BasicTask: task.BasicTask{
										Action: "analyze_sentiment",
									},
								},
								// Deeply nested basic task 2
								{
									BaseConfig: task.BaseConfig{
										ID:   "keyword_extraction",
										Type: task.TaskTypeBasic,
										With: &core.Input{
											"text":              "{{ .workflow.input.content }}",
											"processor_parent":  "{{ .parent.id }}",
											"batch_parent":      "{{ .parent.input.parent_batch }}",
											"original_priority": "{{ .parent.input.parent_priority }}",
											"max_keywords":      "{{ .parent.input.parent_batch | len }}",
										},
									},
									BasicTask: task.BasicTask{
										Action: "extract_keywords",
									},
								},
							},
						},
					},
					// Second sub-task: basic task at first nesting level
					{
						BaseConfig: task.BaseConfig{
							ID:   "metadata_processor",
							Type: task.TaskTypeBasic,
							With: &core.Input{
								"batch_info":     "{{ .parent.input.batch_id }}",
								"batch_priority": "{{ .parent.input.priority }}",
								"parent_task":    "{{ .parent.id }}",
								"workflow_ref":   "{{ .workflow.id }}",
							},
						},
						BasicTask: task.BasicTask{
							Action: "process_metadata",
						},
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "nested-workflow",
			WorkflowExecID: "exec-nested-123",
			Input: &core.Input{
				"content": "This is amazing content to analyze!",
			},
		}

		workflowConfig := &workflow.Config{
			ID:    "nested-workflow",
			Tasks: []task.Config{*nestedParallelTaskConfig},
		}

		// Normalize the nested parallel task structure
		err := normalizer.NormalizeTask(workflowState, workflowConfig, nestedParallelTaskConfig)
		require.NoError(t, err)

		// Verify normalization of the outer parallel task
		assert.Equal(t, "batch-456", (*nestedParallelTaskConfig.With)["batch_id"])
		assert.Equal(
			t,
			float64(100),
			(*nestedParallelTaskConfig.With)["batch_size"],
		) // JSON unmarshaling converts to float64

		// Verify normalization of the first sub-task (nested parallel task)
		dataProcessor := &nestedParallelTaskConfig.Tasks[0]
		assert.Equal(t, "dp-batch-456", (*dataProcessor.With)["processor_id"])
		assert.Equal(t, "batch_processor", (*dataProcessor.With)["parent_batch"])
		assert.Equal(t, "high", (*dataProcessor.With)["parent_priority"])
		assert.Equal(t, "nested-workflow", (*dataProcessor.With)["workflow_id"])

		// Verify normalization of deeply nested basic tasks
		sentimentTask := &dataProcessor.Tasks[0]
		assert.Equal(t, "This is amazing content to analyze!", (*sentimentTask.With)["text"])
		assert.Equal(t, "data_processor", (*sentimentTask.With)["processor_parent"])
		assert.Equal(t, "batch_processor", (*sentimentTask.With)["batch_parent"])
		assert.Equal(t, "high", (*sentimentTask.With)["original_priority"])
		assert.Equal(
			t,
			"nested-workflow.batch_processor.data_processor.sentiment_analysis",
			(*sentimentTask.With)["task_chain"],
		)

		keywordTask := &dataProcessor.Tasks[1]
		assert.Equal(t, "This is amazing content to analyze!", (*keywordTask.With)["text"])
		assert.Equal(t, "data_processor", (*keywordTask.With)["processor_parent"])
		assert.Equal(t, "batch_processor", (*keywordTask.With)["batch_parent"])
		assert.Equal(t, "high", (*keywordTask.With)["original_priority"])
		assert.Equal(t, "15", (*keywordTask.With)["max_keywords"])

		// Verify normalization of the second sub-task (basic task)
		metadataProcessor := &nestedParallelTaskConfig.Tasks[1]
		assert.Equal(t, "batch-456", (*metadataProcessor.With)["batch_info"])
		assert.Equal(t, "high", (*metadataProcessor.With)["batch_priority"])
		assert.Equal(t, "batch_processor", (*metadataProcessor.With)["parent_task"])
		assert.Equal(t, "nested-workflow", (*metadataProcessor.With)["workflow_ref"])
	})

	t.Run("Should handle nested parallel task output access correctly", func(t *testing.T) {
		// Test accessing outputs from deeply nested parallel structures
		aggregatorTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "nested_aggregator",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					// Access nested outputs: batch_processor.data_processor.sentiment_analysis
					"deep_sentiment": "{{ .tasks.batch_processor.output.data_processor.output.sentiment_analysis.output.sentiment }}",
					"deep_keywords":  "{{ .tasks.batch_processor.output.data_processor.output.keyword_extraction.output.keywords }}",
					// Access first-level output: batch_processor.metadata_processor
					"metadata": "{{ .tasks.batch_processor.output.metadata_processor.output.metadata }}",
					// Count nested results
					"keyword_count": "{{ len .tasks.batch_processor.output.data_processor.output.keyword_extraction.output.keywords }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "aggregate_nested",
			},
		}

		// Mock nested parallel task state with deeply nested outputs
		// For deeply nested parallel tasks, the structure should be:
		// batch_processor.output.data_processor.output.sentiment_analysis.output.field
		batchProcessorOutput := map[string]*task.State{
			"data_processor": {
				ExecutionType: task.ExecutionParallel,
				// This simulates how buildParallelTaskOutput would structure nested parallel output
				ParallelState: &task.ParallelState{
					SubTasks: map[string]*task.State{
						"sentiment_analysis": {
							ExecutionType: task.ExecutionBasic,
							Output: &core.Output{
								"sentiment":  "very_positive",
								"confidence": 0.98,
							},
						},
						"keyword_extraction": {
							ExecutionType: task.ExecutionBasic,
							Output: &core.Output{
								"keywords": []string{"amazing", "content", "analyze", "great"},
								"count":    4,
							},
						},
					},
				},
			},
			"metadata_processor": {
				ExecutionType: task.ExecutionBasic,
				Output: &core.Output{
					"metadata":     "processed_metadata",
					"process_time": "1.2s",
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "nested-output-workflow",
			WorkflowExecID: "exec-nested-output",
			Tasks: map[string]*task.State{
				"batch_processor": {
					TaskID:        "batch_processor",
					ExecutionType: task.ExecutionParallel,
					ParallelState: &task.ParallelState{
						SubTasks: batchProcessorOutput,
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "nested-output-workflow",
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "batch_processor",
						Type: task.TaskTypeParallel,
					},
				},
				*aggregatorTaskConfig,
			},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, aggregatorTaskConfig)
		require.NoError(t, err)

		// Verify deeply nested output access
		assert.Equal(t, "very_positive", (*aggregatorTaskConfig.With)["deep_sentiment"])
		assert.Equal(
			t,
			[]string{"amazing", "content", "analyze", "great"},
			(*aggregatorTaskConfig.With)["deep_keywords"],
		)
		assert.Equal(t, "processed_metadata", (*aggregatorTaskConfig.With)["metadata"])
		assert.Equal(t, "4", (*aggregatorTaskConfig.With)["keyword_count"])
	})

	t.Run("Should handle template errors in deeply nested structures gracefully", func(t *testing.T) {
		// Test error handling when templates in deeply nested tasks have issues
		nestedTaskWithError := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "error_batch_processor",
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Tasks: []task.Config{
					{
						BaseConfig: task.BaseConfig{
							ID:   "error_data_processor",
							Type: task.TaskTypeParallel,
						},
						ParallelTask: task.ParallelTask{
							Tasks: []task.Config{
								{
									BaseConfig: task.BaseConfig{
										ID:   "error_task",
										Type: task.TaskTypeBasic,
										With: &core.Input{
											// This should cause an error due to invalid template
											"invalid": "{{ .nonexistent.field.value }}",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "error-workflow",
			WorkflowExecID: "exec-error",
		}

		workflowConfig := &workflow.Config{
			ID:    "error-workflow",
			Tasks: []task.Config{*nestedTaskWithError},
		}

		err := normalizer.NormalizeTask(workflowState, workflowConfig, nestedTaskWithError)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
		assert.Contains(t, err.Error(), "failed to normalize sub-task error_task")
	})
}
