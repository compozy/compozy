package compozy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFunctionsRequireEngineInstance(t *testing.T) {
	t.Parallel()
	var engine *Engine
	tests := []struct {
		name string
		call func(context.Context) error
	}{
		{"LoadProject", func(ctx context.Context) error { return engine.LoadProject(ctx, "config.yaml") }},
		{"LoadProjectsFromDir", func(ctx context.Context) error { return engine.LoadProjectsFromDir(ctx, "configs") }},
		{"LoadWorkflow", func(ctx context.Context) error { return engine.LoadWorkflow(ctx, "workflow.yaml") }},
		{
			"LoadWorkflowsFromDir",
			func(ctx context.Context) error { return engine.LoadWorkflowsFromDir(ctx, "workflows") },
		},
		{"LoadAgent", func(ctx context.Context) error { return engine.LoadAgent(ctx, "agent.yaml") }},
		{"LoadAgentsFromDir", func(ctx context.Context) error { return engine.LoadAgentsFromDir(ctx, "agents") }},
		{"LoadTool", func(ctx context.Context) error { return engine.LoadTool(ctx, "tool.yaml") }},
		{"LoadToolsFromDir", func(ctx context.Context) error { return engine.LoadToolsFromDir(ctx, "tools") }},
		{"LoadKnowledge", func(ctx context.Context) error { return engine.LoadKnowledge(ctx, "knowledge.yaml") }},
		{
			"LoadKnowledgeBasesFromDir",
			func(ctx context.Context) error { return engine.LoadKnowledgeBasesFromDir(ctx, "knowledge") },
		},
		{"LoadMemory", func(ctx context.Context) error { return engine.LoadMemory(ctx, "memory.yaml") }},
		{"LoadMemoriesFromDir", func(ctx context.Context) error { return engine.LoadMemoriesFromDir(ctx, "memories") }},
		{"LoadMCP", func(ctx context.Context) error { return engine.LoadMCP(ctx, "mcp.yaml") }},
		{"LoadMCPsFromDir", func(ctx context.Context) error { return engine.LoadMCPsFromDir(ctx, "mcps") }},
		{"LoadSchema", func(ctx context.Context) error { return engine.LoadSchema(ctx, "schema.yaml") }},
		{"LoadSchemasFromDir", func(ctx context.Context) error { return engine.LoadSchemasFromDir(ctx, "schemas") }},
		{"LoadModel", func(ctx context.Context) error { return engine.LoadModel(ctx, "model.yaml") }},
		{"LoadModelsFromDir", func(ctx context.Context) error { return engine.LoadModelsFromDir(ctx, "models") }},
		{"LoadSchedule", func(ctx context.Context) error { return engine.LoadSchedule(ctx, "schedule.yaml") }},
		{
			"LoadSchedulesFromDir",
			func(ctx context.Context) error { return engine.LoadSchedulesFromDir(ctx, "schedules") },
		},
		{"LoadWebhook", func(ctx context.Context) error { return engine.LoadWebhook(ctx, "webhook.yaml") }},
		{"LoadWebhooksFromDir", func(ctx context.Context) error { return engine.LoadWebhooksFromDir(ctx, "webhooks") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "engine is nil")
		})
	}
}
