package compozy

import (
	"os"
	"path/filepath"
	"testing"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/engine/resources"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestEngineLoaders(t *testing.T) {
	t.Run("Should load single resources", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		baseWorkflow := &engineworkflow.Config{
			ID:    "seed",
			Tasks: []enginetask.Config{{BaseConfig: enginetask.BaseConfig{ID: "task"}}},
		}
		engine, err := New(ctx, WithWorkflow(baseWorkflow))
		require.NoError(t, err)
		engine.resourceStore = resources.NewMemoryResourceStore()
		dir := t.TempDir()

		writeYAML(t, filepath.Join(dir, "project.yaml"), engineproject.Config{Name: "single-project"})
		require.NoError(t, engine.LoadProject(filepath.Join(dir, "project.yaml")))

		writeYAML(t, filepath.Join(dir, "workflow.yaml"), engineworkflow.Config{
			ID:    "wf-single",
			Tasks: []enginetask.Config{{BaseConfig: enginetask.BaseConfig{ID: "task-single"}}},
		})
		require.NoError(t, engine.LoadWorkflow(filepath.Join(dir, "workflow.yaml")))

		writeYAML(t, filepath.Join(dir, "agent.yaml"), engineagent.Config{
			ID:           "agent-single",
			Instructions: "respond helpfully",
			Model: engineagent.Model{
				Config: enginecore.ProviderConfig{
					Provider: enginecore.ProviderName("openai"),
					Model:    "gpt-4o-mini",
				},
			},
		})
		require.NoError(t, engine.LoadAgent(filepath.Join(dir, "agent.yaml")))

		writeYAML(t, filepath.Join(dir, "tool.yaml"), enginetool.Config{ID: "tool-single"})
		require.NoError(t, engine.LoadTool(filepath.Join(dir, "tool.yaml")))

		writeYAML(t, filepath.Join(dir, "knowledge.yaml"), engineknowledge.BaseConfig{ID: "kb-single"})
		require.NoError(t, engine.LoadKnowledge(filepath.Join(dir, "knowledge.yaml")))

		writeYAML(t, filepath.Join(dir, "memory.yaml"), enginememory.Config{ID: "memory-single"})
		require.NoError(t, engine.LoadMemory(filepath.Join(dir, "memory.yaml")))

		writeYAML(t, filepath.Join(dir, "mcp.yaml"), enginemcp.Config{
			ID:        "mcp-single",
			Command:   "echo",
			Transport: mcpproxy.TransportStdio,
		})
		require.NoError(t, engine.LoadMCP(filepath.Join(dir, "mcp.yaml")))

		writeYAML(t, filepath.Join(dir, "schema.yaml"), engineschema.Schema{"id": "schema-single", "type": "object"})
		require.NoError(t, engine.LoadSchema(filepath.Join(dir, "schema.yaml")))

		writeYAML(t, filepath.Join(dir, "model.yaml"), enginecore.ProviderConfig{
			Provider: enginecore.ProviderName("anthropic"),
			Model:    "claude",
		})
		require.NoError(t, engine.LoadModel(filepath.Join(dir, "model.yaml")))

		writeYAML(t, filepath.Join(dir, "schedule.yaml"), projectschedule.Config{
			ID:         "schedule-single",
			WorkflowID: "wf-single",
			Cron:       "*/15 * * * *",
		})
		require.NoError(t, engine.LoadSchedule(filepath.Join(dir, "schedule.yaml")))

		writeYAML(t, filepath.Join(dir, "webhook.yaml"), enginewebhook.Config{
			Slug: "webhook-single",
			Events: []enginewebhook.EventConfig{
				{Name: "created", Filter: "true", Input: map[string]string{"field": "value"}},
			},
		})
		require.NoError(t, engine.LoadWebhook(filepath.Join(dir, "webhook.yaml")))
	})

	t.Run("Should load resources from directories", func(t *testing.T) {
		ctx := lifecycleTestContext(t)
		baseWorkflow := &engineworkflow.Config{
			ID:    "seed-dir",
			Tasks: []enginetask.Config{{BaseConfig: enginetask.BaseConfig{ID: "task-dir"}}},
		}
		engine, err := New(ctx, WithWorkflow(baseWorkflow))
		require.NoError(t, err)
		engine.resourceStore = resources.NewMemoryResourceStore()
		dir := t.TempDir()

		projectDir := filepath.Join(dir, "projects")
		require.NoError(t, os.MkdirAll(projectDir, 0o755))
		writeYAML(t, filepath.Join(projectDir, "project.yaml"), engineproject.Config{Name: "dir-project"})
		require.NoError(t, engine.LoadProjectsFromDir(projectDir))

		workflowDir := filepath.Join(dir, "workflows")
		require.NoError(t, os.MkdirAll(workflowDir, 0o755))
		writeYAML(t, filepath.Join(workflowDir, "workflow.yaml"), engineworkflow.Config{
			ID:    "wf-dir",
			Tasks: []enginetask.Config{{BaseConfig: enginetask.BaseConfig{ID: "task-dir"}}},
		})
		require.NoError(t, engine.LoadWorkflowsFromDir(workflowDir))

		agentDir := filepath.Join(dir, "agents")
		require.NoError(t, os.MkdirAll(agentDir, 0o755))
		writeYAML(t, filepath.Join(agentDir, "agent.yaml"), engineagent.Config{
			ID:           "agent-dir",
			Instructions: "assist users",
			Model: engineagent.Model{
				Config: enginecore.ProviderConfig{
					Provider: enginecore.ProviderName("openai"),
					Model:    "gpt-4o-mini",
				},
			},
		})
		require.NoError(t, engine.LoadAgentsFromDir(agentDir))

		toolDir := filepath.Join(dir, "tools")
		require.NoError(t, os.MkdirAll(toolDir, 0o755))
		writeYAML(t, filepath.Join(toolDir, "tool.yaml"), enginetool.Config{ID: "tool-dir"})
		require.NoError(t, engine.LoadToolsFromDir(toolDir))

		knowledgeDir := filepath.Join(dir, "knowledge")
		require.NoError(t, os.MkdirAll(knowledgeDir, 0o755))
		writeYAML(t, filepath.Join(knowledgeDir, "kb.yaml"), engineknowledge.BaseConfig{ID: "kb-dir"})
		require.NoError(t, engine.LoadKnowledgeBasesFromDir(knowledgeDir))

		memoryDir := filepath.Join(dir, "memories")
		require.NoError(t, os.MkdirAll(memoryDir, 0o755))
		writeYAML(t, filepath.Join(memoryDir, "memory.yaml"), enginememory.Config{ID: "memory-dir"})
		require.NoError(t, engine.LoadMemoriesFromDir(memoryDir))

		mcpDir := filepath.Join(dir, "mcps")
		require.NoError(t, os.MkdirAll(mcpDir, 0o755))
		writeYAML(t, filepath.Join(mcpDir, "mcp.yaml"), enginemcp.Config{
			ID:        "mcp-dir",
			Command:   "echo",
			Transport: mcpproxy.TransportStdio,
		})
		require.NoError(t, engine.LoadMCPsFromDir(mcpDir))

		schemaDir := filepath.Join(dir, "schemas")
		require.NoError(t, os.MkdirAll(schemaDir, 0o755))
		writeYAML(t, filepath.Join(schemaDir, "schema.yaml"), engineschema.Schema{"id": "schema-dir", "type": "object"})
		require.NoError(t, engine.LoadSchemasFromDir(schemaDir))

		modelDir := filepath.Join(dir, "models")
		require.NoError(t, os.MkdirAll(modelDir, 0o755))
		writeYAML(t, filepath.Join(modelDir, "model.yaml"), enginecore.ProviderConfig{
			Provider: enginecore.ProviderName("anthropic"),
			Model:    "claude-3",
		})
		require.NoError(t, engine.LoadModelsFromDir(modelDir))

		scheduleDir := filepath.Join(dir, "schedules")
		require.NoError(t, os.MkdirAll(scheduleDir, 0o755))
		writeYAML(t, filepath.Join(scheduleDir, "schedule.yaml"), projectschedule.Config{
			ID:         "schedule-dir",
			WorkflowID: "wf-dir",
			Cron:       "0 * * * *",
		})
		require.NoError(t, engine.LoadSchedulesFromDir(scheduleDir))

		webhookDir := filepath.Join(dir, "webhooks")
		require.NoError(t, os.MkdirAll(webhookDir, 0o755))
		writeYAML(t, filepath.Join(webhookDir, "webhook.yaml"), enginewebhook.Config{
			Slug: "webhook-dir",
			Events: []enginewebhook.EventConfig{
				{Name: "updated", Filter: "true", Input: map[string]string{"foo": "bar"}},
			},
		})
		require.NoError(t, engine.LoadWebhooksFromDir(webhookDir))
	})
}

func writeYAML(t *testing.T, path string, value any) {
	t.Helper()
	bytes, err := yaml.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, bytes, 0o600))
}
