package compozy

import (
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetask "github.com/compozy/compozy/engine/task"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateReferencesDetectsMissingTask(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "missing"}
	next := "ghost"
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "start",
				OnSuccess: &enginecore.SuccessTransition{Next: &next},
			},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	report, err := engine.ValidateReferences()
	require.NoError(t, err)
	assert.False(t, report.Valid)
	require.NotEmpty(t, report.MissingRefs)
	assert.Contains(t, report.MissingRefs[0].Reference, "ghost")
}

func TestValidateReferencesDetectsCycle(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "cycle"}
	nextB := "b"
	nextA := "a"
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "a",
				OnSuccess: &enginecore.SuccessTransition{Next: &nextB},
			},
		},
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "b",
				OnSuccess: &enginecore.SuccessTransition{Next: &nextA},
			},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	report, err := engine.ValidateReferences()
	require.NoError(t, err)
	assert.False(t, report.Valid)
	require.NotEmpty(t, report.CircularDeps)
	assert.Greater(t, len(report.CircularDeps[0].Chain), 0)
}

func TestRegisterWorkflowPreventsDuplicateIDs(t *testing.T) {
	t.Parallel()
	ctx := lifecycleTestContext(t)
	engine := &Engine{ctx: ctx}
	wf := &engineworkflow.Config{ID: "dup"}
	wf.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{ID: "only"},
		},
	}
	require.NoError(t, engine.RegisterWorkflow(wf))
	err := engine.RegisterWorkflow(wf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegisterSimpleResourcesWarnsOnMissingIDs(t *testing.T) {
	t.Parallel()
	report := &ValidationReport{DependencyGraph: make(map[string][]string)}
	vc := newValidationContext(report)
	vc.registerAgents([]*engineagent.Config{{ID: "agent-1"}, {}})
	require.Len(t, vc.report.Warnings, 1)
	assert.Contains(t, vc.report.Warnings[0].Message, "empty id")
	assert.Contains(t, vc.nodes, "agent:agent-1")
}

func TestRegisterModelsSkipsEmptyEntries(t *testing.T) {
	t.Parallel()
	report := &ValidationReport{DependencyGraph: make(map[string][]string)}
	vc := newValidationContext(report)
	openai := enginecore.ProviderConfig{Provider: enginecore.ProviderName("openai"), Model: "gpt-4o-mini"}
	vc.registerModels([]*enginecore.ProviderConfig{&openai, {}})
	assert.Contains(t, vc.nodes, "model:openai:gpt-4o-mini")
	require.Len(t, vc.report.Warnings, 1)
	assert.Equal(t, "model", vc.report.Warnings[0].ResourceType)
	assert.Contains(t, vc.report.Warnings[0].Message, "empty id")
}

func TestRegisterAdditionalResourcesEmitWarnings(t *testing.T) {
	t.Parallel()
	report := &ValidationReport{DependencyGraph: make(map[string][]string)}
	vc := newValidationContext(report)
	schema := engineschema.Schema{"id": "schema-1", "type": "object"}
	vc.registerMemories([]*enginememory.Config{{ID: "mem-1"}, {}})
	vc.registerMCPs([]*enginemcp.Config{{ID: "mcp-1"}, {}})
	vc.registerSchemas([]*engineschema.Schema{&schema, {}})
	vc.registerSchedules([]*projectschedule.Config{{ID: "schedule-1"}, {}})
	vc.registerWebhooks([]*enginewebhook.Config{{Slug: "hook-1"}, {}})
	assert.Contains(t, vc.nodes, "memory:mem-1")
	assert.Contains(t, vc.nodes, "mcp:mcp-1")
	assert.Contains(t, vc.nodes, "schema:schema-1")
	assert.Contains(t, vc.nodes, "schedule:schedule-1")
	assert.Contains(t, vc.nodes, "webhook:hook-1")
	require.GreaterOrEqual(t, len(vc.report.Warnings), 5)
}

func TestValidationContextAddHelpers(t *testing.T) {
	t.Parallel()
	report := &ValidationReport{DependencyGraph: make(map[string][]string)}
	vc := newValidationContext(report)
	vc.addNode("")
	assert.Empty(t, vc.nodes)
	vc.addEdge("", "target")
	assert.Empty(t, vc.graph)
	vc.addEdge("source", "")
	assert.Empty(t, vc.graph)
	vc.addNode("source")
	vc.addNode("target")
	vc.addEdge("source", "target")
	assert.Len(t, vc.graph["source"], 1)
}
