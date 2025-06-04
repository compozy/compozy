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

		result := n.buildContext(ctx)

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

		result := n.buildContext(ctx)
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
					ID:     "task1",
					Type:   task.TaskTypeBasic,
					Action: "process",
				},
			},
		}

		result := n.buildContext(ctx)
		tasks := result["tasks"].(map[string]any)
		task1 := tasks["task1"].(map[string]any)

		assert.Equal(t, "task1", task1["id"])
		input := task1["input"].(*core.Input)
		assert.Equal(t, "test-data", (*input)["data"])
		output := task1["output"].(*core.Output)
		assert.Equal(t, "processed", (*output)["result"])
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

		result := n.buildContext(ctx)

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
				ID:     "parent-task",
				Type:   task.TaskTypeDecision,
				Action: "decide",
			},
		}

		result := n.buildContext(ctx)
		parent := result["parent"].(map[string]any)

		assert.Equal(t, "parent-task", parent["id"])
		assert.Equal(t, string(task.TaskTypeDecision), parent["type"])
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
			MergedEnv: core.EnvMap{
				"API_KEY": "secret",
			},
		}

		result := n.buildContext(ctx)

		input := result["input"].(*core.Input)
		assert.Equal(t, "value", (*input)["param"])
		assert.Equal(t, "secret", result["env"].(core.EnvMap)["API_KEY"])
	})
}

func TestNormalizer_NormalizeTaskConfig(t *testing.T) {
	n := New()

	t.Run("Should normalize task config with templates", func(t *testing.T) {
		taskConfig := &task.Config{
			ID:     "test-task",
			Action: "process_{{ .workflow.input.city }}",
			With: &core.Input{
				"data":     "{{ .tasks.fetcher.output.data }}",
				"workflow": "{{ .workflow.id }}",
			},
			Env: &core.EnvMap{
				"CITY": "{{ .workflow.input.city | upper }}",
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
			ID:        "decision-task",
			Type:      task.TaskTypeDecision,
			Condition: `{{ eq .tasks.validator.output.status "valid" }}`,
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
			Config: agent.ProviderConfig{Model: agent.ModelGPT4oMini},
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
			MergedEnv: core.EnvMap{
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
			Config: agent.ProviderConfig{
				Model: agent.ModelGPT4o,
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
			MergedEnv: core.EnvMap{
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
			ID: "complex-task",
			With: &core.Input{
				"mode":      `{{ if eq .workflow.input.env "prod" }}production{{ else }}development{{ end }}`,
				"uppercase": "{{ .workflow.input.name | upper }}",
				"length":    "{{ .workflow.input.items | len }}",
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
			ID: "missing-key-task",
			With: &core.Input{
				// This should now fail instead of using default
				"default": "{{ .workflow.input.missing | default \"fallback\" }}",
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
			ID: "nested-task",
			With: &core.Input{
				"author_name":  "{{ .workflow.author.name }}",
				"nested_value": "{{ .tasks.processor.output.data.value }}",
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
			ID: "error-task",
			With: &core.Input{
				"invalid": "{{ .workflow.input.city | nonexistentfunction }}",
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
		assert.Contains(t, err.Error(), "failed to normalize task config input")
	})

	t.Run("Should handle missing context gracefully", func(t *testing.T) {
		taskConfig := &task.Config{
			ID: "missing-context-task",
			With: &core.Input{
				// This should now fail with missingkey=error
				"value": "{{ .tasks.nonexistent.output.data | default \"not-found\" }}",
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
			ID: "typo-task",
			With: &core.Input{
				// This is a common typo: "worklow" instead of "workflow"
				"typo_value": "{{ .worklow.id }}",
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
		assert.Contains(t, err.Error(), "failed to normalize task config input")
	})

	t.Run("Should fail on misspelled field access", func(t *testing.T) {
		taskConfig := &task.Config{
			ID: "misspelled-task",
			With: &core.Input{
				// This is a common typo: "outpu" instead of "output" (missing 't')
				"result": "{{ .tasks.processor.outpu.data }}",
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
		assert.Contains(t, err.Error(), "failed to normalize task config input")
	})
}
