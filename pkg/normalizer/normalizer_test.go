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

func TestNormalizer_BuildContext(t *testing.T) {
	n := New()

	t.Run("Should build context with workflow state", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"city": "New York",
				},
				Output: &core.Output{
					"result": "success",
				},
			},
		}

		result := n.BuildContext(ctx)

		assert.Equal(t, "test-workflow", result["workflow"].(map[string]any)["id"])
		input := result["workflow"].(map[string]any)["input"].(*core.Input)
		assert.Equal(t, "New York", (*input)["city"])
		output := result["workflow"].(map[string]any)["output"].(*core.Output)
		assert.Equal(t, "success", (*output)["result"])
	})

	t.Run("Should build context with workflow config properties", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
				Input:          &core.Input{},
			},
			WorkflowConfig: &workflow.Config{
				ID:          "test-workflow",
				Version:     "1.0.0",
				Description: "Test workflow",
			},
		}

		result := n.BuildContext(ctx)
		wf := result["workflow"].(map[string]any)

		assert.Equal(t, "test-workflow", wf["id"])
		assert.Equal(t, "1.0.0", wf["version"])
		assert.Equal(t, "Test workflow", wf["description"])
	})

	t.Run("Should build context with task states and configs", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"task1": {
						Input: &core.Input{
							"data": "test-data",
						},
						Output: &core.Output{
							"result": "processed",
						},
					},
				},
			},
			TaskConfigs: map[string]*task.Config{
				"task1": {
					BaseConfig: task.BaseConfig{
						ID:   "task1",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "process",
					},
				},
			},
		}

		result := n.BuildContext(ctx)
		tasks := result["tasks"].(map[string]any)
		task1 := tasks["task1"].(map[string]any)

		assert.Equal(t, "task1", task1["id"])
		input := task1["input"].(*core.Input)
		assert.Equal(t, "test-data", (*input)["data"])
		output := task1["output"].(core.Output)
		assert.Equal(t, "processed", output["result"])
		assert.Equal(t, string(task.TaskTypeBasic), task1["type"])
		assert.Equal(t, "process", task1["action"])
	})

	t.Run("Should build context with parent config", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
			},
			ParentConfig: map[string]any{
				"id":     "parent-task",
				"type":   "basic",
				"action": "process",
			},
		}

		result := n.BuildContext(ctx)

		assert.Equal(t, "parent-task", result["parent"].(map[string]any)["id"])
		assert.Equal(t, "basic", result["parent"].(map[string]any)["type"])
		assert.Equal(t, "process", result["parent"].(map[string]any)["action"])
	})

	t.Run("Should build context with parent task config", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"parent-task": {
						Input: &core.Input{
							"city": "Boston",
						},
						Output: &core.Output{
							"status": "complete",
						},
					},
				},
			},
			ParentTaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "parent-task",
					Type: task.TaskTypeBasic,
				},
				BasicTask: task.BasicTask{
					Action: "decide",
				},
			},
		}

		result := n.BuildContext(ctx)
		parent := result["parent"].(map[string]any)

		assert.Equal(t, "parent-task", parent["id"])
		assert.Equal(t, string(task.TaskTypeBasic), parent["type"])
		assert.Equal(t, "decide", parent["action"])
		input := parent["input"].(*core.Input)
		assert.Equal(t, "Boston", (*input)["city"])
		output := parent["output"].(*core.Output)
		assert.Equal(t, "complete", (*output)["status"])
	})

	t.Run("Should include current input and env", func(t *testing.T) {
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
			},
			CurrentInput: &core.Input{
				"param": "value",
			},
			MergedEnv: &core.EnvMap{
				"API_KEY": "secret",
			},
		}

		result := n.BuildContext(ctx)

		input := result["input"].(*core.Input)
		assert.Equal(t, "value", (*input)["param"])
		assert.Equal(t, "secret", (*result["env"].(*core.EnvMap))["API_KEY"])
	})
}

func TestNormalizer_NormalizeTaskConfig(t *testing.T) {
	n := New()

	t.Run("Should normalize task config with templates", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"data":     "{{ .tasks.fetcher.output.data }}",
					"workflow": "{{ .workflow.id }}",
				},
				Env: &core.EnvMap{
					"CITY": "{{ .workflow.input.city | upper }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "process_{{ .workflow.input.city }}",
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"city": "seattle",
				},
				Tasks: map[string]*task.State{
					"fetcher": {
						Output: &core.Output{
							"data": "fetched-data",
						},
					},
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.NoError(t, err)

		assert.Equal(t, "process_seattle", taskConfig.Action)
		assert.Equal(t, "fetched-data", (*taskConfig.With)["data"])
		assert.Equal(t, "my-workflow", (*taskConfig.With)["workflow"])
		assert.Equal(t, "SEATTLE", taskConfig.GetEnv().Prop("CITY"))
	})

	t.Run("Should handle condition field normalization", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "router-task",
				Type:      task.TaskTypeRouter,
				Condition: `{{ eq .tasks.validator.output.status "valid" }}`,
			},
			RouterTask: task.RouterTask{},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"validator": {
						Output: &core.Output{
							"status": "valid",
						},
					},
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.NoError(t, err)

		assert.Equal(t, "true", taskConfig.Condition)
	})

	t.Run("Should not process outputs field during config normalization", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-with-outputs",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"city": "{{ .workflow.input.city }}",
				},
				// This should NOT be processed during config normalization since there's no output yet
				Outputs: &core.Input{
					"temperature": "{{ .output.temperature }}",
					"humidity":    "{{ .output.humidity }}",
				},
			},
			BasicTask: task.BasicTask{
				Action: "get_weather",
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "weather-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"city": "Seattle",
				},
				// No task output yet - this is during config normalization
			},
		}

		// This should succeed because outputs field is excluded from config normalization
		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.NoError(t, err)

		// The outputs field should remain as template strings (not processed)
		assert.Equal(t, "{{ .output.temperature }}", (*taskConfig.Outputs)["temperature"])
		assert.Equal(t, "{{ .output.humidity }}", (*taskConfig.Outputs)["humidity"])

		// The regular config fields should be processed normally
		assert.Equal(t, "Seattle", (*taskConfig.With)["city"])
	})

	t.Run("Should not process outputs field in parallel tasks during config normalization", func(t *testing.T) {
		parallelTaskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parallel-with-outputs",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"city": "{{ .workflow.input.city }}",
				},
				// This should NOT be processed during config normalization
				Outputs: &core.Input{
					"combined_result": "{{ .output.sentiment_analysis.output.sentiment }} + {{ .output.keyword_extraction.output.keywords }}",
				},
			},
			ParallelTask: task.ParallelTask{
				Strategy: task.StrategyWaitAll,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "sentiment_analysis",
						Type: task.TaskTypeBasic,
					},
					BasicTask: task.BasicTask{
						Action: "analyze",
					},
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "parallel-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"city": "Boston",
				},
			},
		}

		// This should succeed because outputs field is excluded from config normalization
		err := n.NormalizeTaskConfig(parallelTaskConfig, ctx)
		require.NoError(t, err)

		// The outputs field should remain as template strings (not processed)
		assert.Equal(
			t,
			"{{ .output.sentiment_analysis.output.sentiment }} + {{ .output.keyword_extraction.output.keywords }}",
			(*parallelTaskConfig.Outputs)["combined_result"],
		)

		// The regular config fields should be processed normally
		assert.Equal(t, "Boston", (*parallelTaskConfig.With)["city"])
	})
}

func TestNormalizer_NormalizeAgentConfig(t *testing.T) {
	n := New()

	t.Run("Should normalize agent config with templates", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "test-agent",
			Instructions: `Process data for {{ .parent.input.city }}.
Workflow: {{ .workflow.id }}
Parent task: {{ .parent.id }}`,
			With: &core.Input{
				"context": "{{ .tasks.context_builder.output.context }}",
			},
			Env: &core.EnvMap{
				"AGENT_MODE": "{{ .env.MODE | default \"production\" }}",
			},
			Config: core.ProviderConfig{Model: "gpt-4o-mini"},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"context_builder": {
						Output: &core.Output{
							"context": "generated-context",
						},
					},
				},
			},
			ParentConfig: map[string]any{
				"id": "caller-task",
				"input": &core.Input{
					"city": "Portland",
				},
			},
			MergedEnv: &core.EnvMap{
				"MODE": "debug",
			},
		}

		err := n.NormalizeAgentConfig(agentConfig, ctx, "test-action")
		require.NoError(t, err)

		expectedInstructions := `Process data for Portland.
Workflow: my-workflow
Parent task: caller-task`

		assert.Equal(t, expectedInstructions, agentConfig.Instructions)
		assert.Equal(t, "generated-context", (*agentConfig.With)["context"])
		assert.Equal(t, "debug", agentConfig.GetEnv().Prop("AGENT_MODE"))
	})

	t.Run("Should normalize agent actions and access parent agent config", func(t *testing.T) {
		agentConfig := &agent.Config{
			ID: "test-agent-for-actions",
			Config: core.ProviderConfig{
				Model: "gpt-4o",
			},
			With: &core.Input{
				"data":      "test-data-for-action",
				"threshold": "0.95",
			},
			Actions: []*agent.ActionConfig{
				{
					ID:     "action1",
					Prompt: "Analyze {{ .parent.input.data }} for {{ .parent.id }} using model {{ .parent.config.Model }}",
					With: &core.Input{
						"threshold": "{{ .parent.input.threshold | default \"0.8\" }}",
					},
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow-for-actions",
				WorkflowExecID: "exec-actions-123",
			},
		}

		err := n.NormalizeAgentConfig(agentConfig, ctx, "action1")
		require.NoError(t, err)

		action := agentConfig.Actions[0]
		expectedPrompt := "Analyze test-data-for-action for test-agent-for-actions using model gpt-4o"
		assert.Equal(t, expectedPrompt, action.Prompt)
		assert.Equal(t, "0.95", (*action.With)["threshold"])
	})
}

func TestNormalizer_NormalizeToolConfig(t *testing.T) {
	n := New()

	t.Run("Should normalize tool config with templates", func(t *testing.T) {
		toolConfig := &tool.Config{
			ID:          "test-tool",
			Execute:     "{{ .env.SCRIPTS_PATH }}/process.ts",
			Description: "Tool for {{ .parent.id }} task",
			With: &core.Input{
				"endpoint": "{{ .env.API_URL }}/{{ .parent.action }}",
				"city":     "{{ .parent.input.city }}",
			},
			Env: &core.EnvMap{
				"TOOL_MODE": "{{ .parent.type }}_mode",
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
			},
			ParentConfig: map[string]any{
				"id":     "api-task",
				"type":   "basic",
				"action": "fetch",
				"input": &core.Input{
					"city": "Chicago",
				},
			},
			MergedEnv: &core.EnvMap{
				"SCRIPTS_PATH": "/scripts",
				"API_URL":      "https://api.example.com",
			},
		}

		err := n.NormalizeToolConfig(toolConfig, ctx)
		require.NoError(t, err)

		assert.Equal(t, "/scripts/process.ts", toolConfig.Execute)
		assert.Equal(t, "Tool for api-task task", toolConfig.Description)
		assert.Equal(t, "https://api.example.com/fetch", (*toolConfig.With)["endpoint"])
		assert.Equal(t, "Chicago", (*toolConfig.With)["city"])
		assert.Equal(t, "basic_mode", toolConfig.GetEnv().Prop("TOOL_MODE"))
	})
}

func TestNormalizer_ComplexTemplates(t *testing.T) {
	n := New()

	t.Run("Should handle complex template expressions", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "complex-task",
				With: &core.Input{
					"mode":      `{{ if eq .workflow.input.env "prod" }}production{{ else }}development{{ end }}`,
					"uppercase": "{{ .workflow.input.name | upper }}",
					"length":    "{{ .workflow.input.items | len }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"env":   "prod",
					"name":  "test",
					"items": []string{"a", "b", "c"},
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.NoError(t, err)

		assert.Equal(t, "production", (*taskConfig.With)["mode"])
		assert.Equal(t, "TEST", (*taskConfig.With)["uppercase"])
		assert.Equal(t, "3", (*taskConfig.With)["length"])
	})

	t.Run("Should handle missing key with error", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "missing-key-task",
				With: &core.Input{
					// This should now fail instead of using default
					"default": "{{ .workflow.input.missing | default \"fallback\" }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"env":   "prod",
					"name":  "test",
					"items": []string{"a", "b", "c"},
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing")
	})

	t.Run("Should handle nested object access", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "nested-task",
				With: &core.Input{
					"author_name":  "{{ .workflow.author.name }}",
					"nested_value": "{{ .tasks.processor.output.data.value }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"processor": {
						Output: &core.Output{
							"data": map[string]any{
								"value": "nested-result",
							},
						},
					},
				},
			},
			WorkflowConfig: &workflow.Config{
				Author: &core.Author{
					Name: "John Doe",
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", (*taskConfig.With)["author_name"])
		assert.Equal(t, "nested-result", (*taskConfig.With)["nested_value"])
	})
}

func TestNormalizer_ErrorHandling(t *testing.T) {
	n := New()

	t.Run("Should return error for invalid template", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "error-task",
				With: &core.Input{
					"invalid": "{{ .workflow.input.city | nonexistentfunction }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Input: &core.Input{
					"city": "test",
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to normalize task config")
	})

	t.Run("Should handle missing context gracefully", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "missing-context-task",
				With: &core.Input{
					// This should now fail with missingkey=error
					"value": "{{ .tasks.nonexistent.output.data | default \"not-found\" }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
	})

	t.Run("Should fail fast on typo that was previously silent", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "typo-task",
				With: &core.Input{
					// This is a common typo: "worklow" instead of "workflow"
					"typo_value": "{{ .worklow.id }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "worklow")
		assert.Contains(t, err.Error(), "failed to normalize task config")
	})

	t.Run("Should fail on misspelled field access", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "misspelled-task",
				With: &core.Input{
					// This is a common typo: "outpu" instead of "output" (missing 't')
					"result": "{{ .tasks.processor.outpu.data }}",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "my-workflow",
				WorkflowExecID: "exec-123",
				Tasks: map[string]*task.State{
					"processor": {
						Output: &core.Output{
							"data": "processed-result",
						},
					},
				},
			},
		}

		err := n.NormalizeTaskConfig(taskConfig, ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outpu")
		assert.Contains(t, err.Error(), "failed to normalize task config")
	})
}

func TestNormalizer_NormalizeTaskConfigWithSignal(t *testing.T) {
	n := New()

	t.Run("Should normalize task config with signal context", func(t *testing.T) {
		// Create a task config with signal template
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-processor",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"message": "Processing signal: {{ .signal.payload.status }}",
					"data":    "{{ .signal.payload.data }}",
				},
			},
		}

		// Create signal data
		signal := &task.SignalEnvelope{
			Metadata: task.SignalMetadata{
				SignalID:   "test-signal",
				WorkflowID: "test-workflow",
				Source:     "test-source",
			},
			Payload: map[string]any{
				"status": "ready",
				"data":   "test-data",
			},
		}

		// Create normalization context
		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: "exec-123",
				Input:          &core.Input{},
				Output:         &core.Output{},
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfigs: make(map[string]*task.Config),
		}

		// Normalize with signal context
		err := n.NormalizeTaskConfigWithSignal(taskConfig, ctx, signal)
		require.NoError(t, err)

		// Verify signal templates were resolved
		assert.Equal(t, "Processing signal: ready", (*taskConfig.With)["message"])
		assert.Equal(t, "test-data", (*taskConfig.With)["data"])
	})

	t.Run("Should handle missing signal properties gracefully", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-processor",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"message": "Signal received with other: {{ .signal.payload.other }}",
				},
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"other": "value",
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
				Input:      &core.Input{},
				Output:     &core.Output{},
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfigs: make(map[string]*task.Config),
		}

		err := n.NormalizeTaskConfigWithSignal(taskConfig, ctx, signal)
		require.NoError(t, err)

		// Should access available property
		assert.Equal(t, "Signal received with other: value", (*taskConfig.With)["message"])
	})

	t.Run("Should handle nil signal gracefully", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-processor",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"message": "Static message",
				},
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
				Input:      &core.Input{},
				Output:     &core.Output{},
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfigs: make(map[string]*task.Config),
		}

		err := n.NormalizeTaskConfigWithSignal(taskConfig, ctx, nil)
		require.NoError(t, err)

		// Should preserve static content
		assert.Equal(t, "Static message", (*taskConfig.With)["message"])
	})

	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		signal := &task.SignalEnvelope{}
		ctx := &NormalizationContext{}

		err := n.NormalizeTaskConfigWithSignal(nil, ctx, signal)
		require.NoError(t, err)
	})

	t.Run("Should preserve existing With values after normalization", func(t *testing.T) {
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-processor",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"static":   "unchanged",
					"dynamic":  "{{ .signal.payload.status }}",
					"existing": "preserved",
				},
			},
		}

		signal := &task.SignalEnvelope{
			Payload: map[string]any{
				"status": "active",
			},
		}

		ctx := &NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
				Input:      &core.Input{},
				Output:     &core.Output{},
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfigs: make(map[string]*task.Config),
		}

		err := n.NormalizeTaskConfigWithSignal(taskConfig, ctx, signal)
		require.NoError(t, err)

		// Should have normalized dynamic content
		assert.Equal(t, "active", (*taskConfig.With)["dynamic"])
		// Should preserve static content
		assert.Equal(t, "unchanged", (*taskConfig.With)["static"])
		assert.Equal(t, "preserved", (*taskConfig.With)["existing"])
	})
}
