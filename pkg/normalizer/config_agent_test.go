package normalizer

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
