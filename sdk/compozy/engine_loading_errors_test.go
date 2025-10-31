package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFunctionsRequireEngineInstance(t *testing.T) {
	t.Parallel()
	var engine *Engine
	tests := []struct {
		name string
		call func() error
	}{
		{"LoadProject", func() error { return engine.LoadProject("config.yaml") }},
		{"LoadProjectsFromDir", func() error { return engine.LoadProjectsFromDir("configs") }},
		{"LoadWorkflow", func() error { return engine.LoadWorkflow("workflow.yaml") }},
		{"LoadWorkflowsFromDir", func() error { return engine.LoadWorkflowsFromDir("workflows") }},
		{"LoadAgent", func() error { return engine.LoadAgent("agent.yaml") }},
		{"LoadAgentsFromDir", func() error { return engine.LoadAgentsFromDir("agents") }},
		{"LoadTool", func() error { return engine.LoadTool("tool.yaml") }},
		{"LoadToolsFromDir", func() error { return engine.LoadToolsFromDir("tools") }},
		{"LoadKnowledge", func() error { return engine.LoadKnowledge("knowledge.yaml") }},
		{"LoadKnowledgeBasesFromDir", func() error { return engine.LoadKnowledgeBasesFromDir("knowledge") }},
		{"LoadMemory", func() error { return engine.LoadMemory("memory.yaml") }},
		{"LoadMemoriesFromDir", func() error { return engine.LoadMemoriesFromDir("memories") }},
		{"LoadMCP", func() error { return engine.LoadMCP("mcp.yaml") }},
		{"LoadMCPsFromDir", func() error { return engine.LoadMCPsFromDir("mcps") }},
		{"LoadSchema", func() error { return engine.LoadSchema("schema.yaml") }},
		{"LoadSchemasFromDir", func() error { return engine.LoadSchemasFromDir("schemas") }},
		{"LoadModel", func() error { return engine.LoadModel("model.yaml") }},
		{"LoadModelsFromDir", func() error { return engine.LoadModelsFromDir("models") }},
		{"LoadSchedule", func() error { return engine.LoadSchedule("schedule.yaml") }},
		{"LoadSchedulesFromDir", func() error { return engine.LoadSchedulesFromDir("schedules") }},
		{"LoadWebhook", func() error { return engine.LoadWebhook("webhook.yaml") }},
		{"LoadWebhooksFromDir", func() error { return engine.LoadWebhooksFromDir("webhooks") }},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "engine is nil")
		})
	}
}
