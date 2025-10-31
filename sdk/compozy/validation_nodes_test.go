package compozy

import (
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowValidationNodes(t *testing.T) {
	t.Run("Should register workflow-level resources in dependency graph", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		engine, err := New(
			ctx,
			WithWorkflow(
				&engineworkflow.Config{
					ID:    "seed",
					Tasks: []enginetask.Config{{BaseConfig: enginetask.BaseConfig{ID: "seed-task"}}},
				},
			),
		)
		require.NoError(t, err)

		require.NoError(t, engine.RegisterProject(&engineproject.Config{Name: "graph-project"}))
		require.NoError(
			t,
			engine.RegisterAgent(
				&engineagent.Config{
					ID:           "task-agent",
					Instructions: "assist",
					Model: engineagent.Model{
						Config: enginecore.ProviderConfig{
							Provider: enginecore.ProviderName("openai"),
							Model:    "gpt-4o-mini",
						},
					},
				},
			),
		)
		require.NoError(t, engine.RegisterTool(&enginetool.Config{ID: "task-tool"}))
		require.NoError(t, engine.RegisterKnowledge(&engineknowledge.BaseConfig{ID: "task-kb"}))

		next := "step-final"
		wf := &engineworkflow.Config{
			ID:             "graph-workflow",
			Agents:         []engineagent.Config{{ID: "workflow-agent"}},
			Tools:          []enginetool.Config{{ID: "workflow-tool"}},
			KnowledgeBases: []engineknowledge.BaseConfig{{ID: "workflow-kb"}},
			Tasks: []enginetask.Config{
				{
					BaseConfig: enginetask.BaseConfig{
						ID:        "step-start",
						Agent:     &engineagent.Config{ID: "task-agent"},
						OnSuccess: &enginecore.SuccessTransition{Next: &next},
					},
				},
				{
					BaseConfig: enginetask.BaseConfig{
						ID:   "step-final",
						Tool: &enginetool.Config{ID: "task-tool"},
					},
				},
			},
		}
		require.NoError(t, engine.RegisterWorkflow(wf))

		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.True(t, report.Valid)
		assert.Contains(t, report.DependencyGraph, "workflow:graph-workflow")
		taskDeps, ok := report.DependencyGraph["task:graph-workflow/step-start"]
		require.True(t, ok)
		assert.Contains(t, taskDeps, "agent:task-agent")
		assert.GreaterOrEqual(t, report.ResourceCount, 4)
		assert.Empty(t, report.Warnings)
	})
}
