package compozy

import (
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterProject(t *testing.T) {
	t.Run("Should register project and reject duplicates", func(t *testing.T) {
		engine := newSeedEngine(t)
		require.NoError(t, engine.RegisterProject(&engineproject.Config{Name: "reg-project"}))
		assert.Equal(t, "reg-project", engine.project.Name)
		assert.Error(t, engine.RegisterProject(&engineproject.Config{Name: "reg-project"}))
	})
}

func TestRegisterWorkflow(t *testing.T) {
	t.Run("Should register secondary workflow", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterWorkflow(&engineworkflow.Config{
			ID: "secondary",
			Tasks: []enginetask.Config{
				{BaseConfig: enginetask.BaseConfig{ID: "secondary-task"}},
			},
		}))
		assert.Len(t, engine.workflows, 2)
	})
}

func TestRegisterAgent(t *testing.T) {
	t.Run("Should register agent and detect duplicate", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterAgent(&engineagent.Config{
			ID:           "agent-alpha",
			Instructions: "Provide assistance",
			Model: engineagent.Model{
				Config: enginecore.ProviderConfig{
					Provider: enginecore.ProviderName("openai"),
					Model:    "gpt-4o-mini",
				},
			},
		}))
		assert.Error(t, engine.RegisterAgent(&engineagent.Config{ID: "agent-alpha"}))
		assert.Len(t, engine.agents, 1)
	})
}

func TestRegisterTool(t *testing.T) {
	t.Run("Should register tool", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterTool(&enginetool.Config{ID: "tool-alpha"}))
		assert.Len(t, engine.tools, 1)
		assert.Error(t, engine.RegisterTool(&enginetool.Config{ID: "tool-alpha"}))
	})
}

func TestRegisterKnowledgeBase(t *testing.T) {
	t.Run("Should register knowledge base", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterKnowledge(&engineknowledge.BaseConfig{ID: "kb-alpha"}))
		assert.Len(t, engine.knowledgeBases, 1)
		assert.Error(t, engine.RegisterKnowledge(&engineknowledge.BaseConfig{ID: "kb-alpha"}))
	})
}

func TestRegisterMemory(t *testing.T) {
	t.Run("Should register memory", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterMemory(&enginememory.Config{ID: "memory-alpha"}))
		assert.Len(t, engine.memories, 1)
		assert.Error(t, engine.RegisterMemory(&enginememory.Config{ID: "memory-alpha"}))
	})
}

func TestRegisterMCP(t *testing.T) {
	t.Run("Should register mcp", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterMCP(&enginemcp.Config{
			ID:        "mcp-alpha",
			Command:   "echo",
			Transport: mcpproxy.TransportStdio,
		}))
		assert.Len(t, engine.mcps, 1)
		assert.Error(t, engine.RegisterMCP(&enginemcp.Config{ID: "mcp-alpha"}))
	})
}

func TestRegisterSchema(t *testing.T) {
	t.Run("Should register schema", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		schema := engineschema.Schema{"id": "schema-alpha", "type": "object"}
		require.NoError(t, engine.RegisterSchema(&schema))
		assert.Len(t, engine.schemas, 1)
		assert.Error(t, engine.RegisterSchema(&schema))
	})
}

func TestRegisterModel(t *testing.T) {
	t.Run("Should register model", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterModel(&enginecore.ProviderConfig{
			Provider: enginecore.ProviderName("anthropic"),
			Model:    "claude",
		}))
		assert.Len(t, engine.models, 1)
		assert.Error(t, engine.RegisterModel(&enginecore.ProviderConfig{
			Provider: enginecore.ProviderName("anthropic"),
			Model:    "claude",
		}))
	})
}

func TestRegisterSchedule(t *testing.T) {
	t.Run("Should register schedule", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterWorkflow(&engineworkflow.Config{
			ID: "secondary",
			Tasks: []enginetask.Config{
				{BaseConfig: enginetask.BaseConfig{ID: "secondary-task"}},
			},
		}))
		require.NoError(t, engine.RegisterSchedule(&projectschedule.Config{
			ID:         "schedule-alpha",
			WorkflowID: "secondary",
			Cron:       "*/5 * * * *",
		}))
		assert.Len(t, engine.schedules, 1)
		assert.Error(t, engine.RegisterSchedule(&projectschedule.Config{
			ID:         "schedule-alpha",
			WorkflowID: "secondary",
			Cron:       "*/5 * * * *",
		}))
	})
}

func TestRegisterWebhook(t *testing.T) {
	t.Run("Should register webhook", func(t *testing.T) {
		engine := newSeedEngine(t)
		requireProjectRegistered(t, engine, "reg-project")
		require.NoError(t, engine.RegisterWebhook(&enginewebhook.Config{
			Slug: "webhook-alpha",
			Events: []enginewebhook.EventConfig{
				{
					Name:   "created",
					Filter: "true",
					Input:  map[string]string{"field": "value"},
				},
			},
		}))
		assert.Len(t, engine.webhooks, 1)
		assert.Error(t, engine.RegisterWebhook(&enginewebhook.Config{Slug: "webhook-alpha"}))
	})
}

func newSeedEngine(t *testing.T) *Engine {
	t.Helper()
	ctx := lifecycleTestContext(t)
	baseWorkflow := &engineworkflow.Config{
		ID: "seed",
		Tasks: []enginetask.Config{
			{BaseConfig: enginetask.BaseConfig{ID: "seed-task"}},
		},
	}
	engine, err := New(ctx, WithWorkflow(baseWorkflow))
	require.NoError(t, err)
	engine.resourceStore = resources.NewMemoryResourceStore()
	return engine
}

func requireProjectRegistered(t *testing.T, engine *Engine, name string) {
	t.Helper()
	require.NoError(t, engine.RegisterProject(&engineproject.Config{Name: name}))
}
