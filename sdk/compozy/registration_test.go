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

func TestEngineRegisterResources(t *testing.T) {
	t.Run("Should register all resource types", func(t *testing.T) {
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

		require.NoError(t, engine.RegisterProject(&engineproject.Config{Name: "reg-project"}))
		require.Error(t, engine.RegisterProject(&engineproject.Config{Name: "reg-project"}))

		require.NoError(t, engine.RegisterWorkflow(&engineworkflow.Config{
			ID: "secondary",
			Tasks: []enginetask.Config{
				{BaseConfig: enginetask.BaseConfig{ID: "secondary-task"}},
			},
		}))

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
		require.Error(t, engine.RegisterAgent(&engineagent.Config{ID: "agent-alpha"}))

		require.NoError(t, engine.RegisterTool(&enginetool.Config{ID: "tool-alpha"}))
		require.NoError(t, engine.RegisterKnowledge(&engineknowledge.BaseConfig{ID: "kb-alpha"}))
		require.NoError(t, engine.RegisterMemory(&enginememory.Config{ID: "memory-alpha"}))
		require.NoError(t, engine.RegisterMCP(&enginemcp.Config{
			ID:        "mcp-alpha",
			Command:   "echo",
			Transport: mcpproxy.TransportStdio,
		}))
		require.NoError(t, engine.RegisterSchema(&engineschema.Schema{"id": "schema-alpha", "type": "object"}))
		require.NoError(t, engine.RegisterModel(&enginecore.ProviderConfig{
			Provider: enginecore.ProviderName("anthropic"),
			Model:    "claude",
		}))
		require.NoError(t, engine.RegisterSchedule(&projectschedule.Config{
			ID:         "schedule-alpha",
			WorkflowID: "secondary",
			Cron:       "*/5 * * * *",
		}))
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

		assert.Equal(t, "reg-project", engine.project.Name)
		assert.Len(t, engine.workflows, 2)
		assert.Len(t, engine.agents, 1)
		assert.Len(t, engine.tools, 1)
		assert.Len(t, engine.knowledgeBases, 1)
		assert.Len(t, engine.memories, 1)
		assert.Len(t, engine.mcps, 1)
		assert.Len(t, engine.schemas, 1)
		assert.Len(t, engine.models, 1)
		assert.Len(t, engine.schedules, 1)
		assert.Len(t, engine.webhooks, 1)
	})
}
