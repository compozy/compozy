package normalizer

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigNormalizer_NormalizeToolComponent(t *testing.T) {
	normalizer := NewConfigNormalizer()

	t.Run("Should normalize tool with complete parent context and environment merging", func(t *testing.T) {
		toolConfig := &tool.Config{
			ID: "api-caller",
			// Execute field removed - tools resolved via entrypoint exports
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
		// Execute field removed - tools resolved via entrypoint exports
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
