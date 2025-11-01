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
		engine := newWorkflowValidationEngine(t)
		registerGraphWorkflow(t, engine)
		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.True(t, report.Valid)
		assert.Contains(t, report.DependencyGraph, "workflow:graph-workflow")
		taskDeps, ok := report.DependencyGraph["task:graph-workflow/step-start"]
		require.True(t, ok)
		assert.Contains(t, taskDeps, "agent:task-agent")
		assert.Contains(t, taskDeps, "task:graph-workflow/step-final")
		finalDeps, ok := report.DependencyGraph["task:graph-workflow/step-final"]
		require.True(t, ok)
		assert.Contains(t, finalDeps, "tool:task-tool")
		assert.GreaterOrEqual(t, report.ResourceCount, 4)
		assert.Empty(t, report.Warnings)
	})
}

func TestWorkflowValidationMissingReferences(t *testing.T) {
	t.Run("Should report missing agent when task binding unresolved", func(t *testing.T) {
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
		next := "step-final"
		workflow := &engineworkflow.Config{
			ID: "graph-workflow",
			Tasks: []enginetask.Config{
				{
					BaseConfig: enginetask.BaseConfig{
						ID:        "step-start",
						Agent:     &engineagent.Config{ID: "task-agent"},
						OnSuccess: &enginecore.SuccessTransition{Next: &next},
					},
				},
			},
		}
		require.NoError(t, engine.RegisterWorkflow(workflow))
		report, err := engine.ValidateReferences()
		require.NoError(t, err)
		assert.False(t, report.Valid)
		references := make([]string, len(report.MissingRefs))
		for i := range report.MissingRefs {
			references[i] = report.MissingRefs[i].Reference
		}
		assert.Contains(t, references, "agent:task-agent")
	})
}

func TestWorkflowValidationWarnings(t *testing.T) {
	t.Run("Should emit warnings for workflow resources with empty identifiers", func(t *testing.T) {
		report := &ValidationReport{Warnings: make([]ValidationWarning, 0)}
		vc := newValidationContext(report)
		vc.registerWorkflowAgents("wf-empty", []engineagent.Config{{ID: " "}})
		vc.registerWorkflowTools("wf-empty", []enginetool.Config{{ID: ""}})
		vc.registerWorkflowKnowledge("wf-empty", []engineknowledge.BaseConfig{{ID: ""}})
		require.Len(t, report.Warnings, 3)
		assert.Contains(t, report.Warnings[0].Message, "agent with empty id")
		assert.Contains(t, report.Warnings[1].Message, "workflow tool with empty id")
		assert.Contains(t, report.Warnings[2].Message, "knowledge base with empty id")
	})
}

func newWorkflowValidationEngine(t *testing.T) *Engine {
	t.Helper()
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
	return engine
}

func registerGraphWorkflow(t *testing.T, engine *Engine) {
	t.Helper()
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
}
