package compozy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
)

func TestDefaultConfigInitializesCollections(t *testing.T) {
	t.Parallel()
	cfg := defaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, defaultMode, cfg.mode)
	assert.Equal(t, defaultHost, cfg.host)
	assert.Empty(t, cfg.workflows)
	assert.Empty(t, cfg.agents)
	assert.Empty(t, cfg.tools)
	assert.Empty(t, cfg.knowledgeBases)
	assert.Empty(t, cfg.memories)
	assert.Empty(t, cfg.mcps)
	assert.Empty(t, cfg.schemas)
	assert.Empty(t, cfg.models)
	assert.Empty(t, cfg.schedules)
	assert.Empty(t, cfg.webhooks)
}

func TestOptionsApplyBasics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		option Option
		check  func(*config)
	}{
		{
			name:   "WithMode",
			option: WithMode(ModeDistributed),
			check: func(cfg *config) {
				assert.Equal(t, ModeDistributed, cfg.mode)
			},
		},
		{
			name:   "WithHost",
			option: WithHost(" 0.0.0.0 "),
			check: func(cfg *config) {
				assert.Equal(t, "0.0.0.0", cfg.host)
			},
		},
		{
			name:   "WithPort",
			option: WithPort(8080),
			check: func(cfg *config) {
				assert.Equal(t, 8080, cfg.port)
			},
		},
		{
			name: "WithProject",
			option: func() Option {
				projectCfg := &engineproject.Config{Name: "demo"}
				return WithProject(projectCfg)
			}(),
			check: func(cfg *config) {
				require.NotNil(t, cfg.project)
				assert.Equal(t, "demo", cfg.project.Name)
			},
		},
	}
	for _, tc := range tests {
		caseEntry := tc
		t.Run(caseEntry.name, func(t *testing.T) {
			applyAndCheckOption(t, caseEntry.option, caseEntry.check)
		})
	}
}

func TestWithWorkflowOption(t *testing.T) {
	t.Parallel()
	cfg := &engineworkflow.Config{ID: "wf"}
	applyAndCheckOption(t, WithWorkflow(cfg), func(c *config) {
		require.Len(t, c.workflows, 1)
		assert.Equal(t, "wf", c.workflows[0].ID)
	})
}

func TestWithAgentOption(t *testing.T) {
	t.Parallel()
	cfg := &engineagent.Config{ID: "agent"}
	applyAndCheckOption(t, WithAgent(cfg), func(c *config) {
		require.Len(t, c.agents, 1)
		assert.Equal(t, "agent", c.agents[0].ID)
	})
}

func TestWithToolOption(t *testing.T) {
	t.Parallel()
	cfg := &enginetool.Config{ID: "tool"}
	applyAndCheckOption(t, WithTool(cfg), func(c *config) {
		require.Len(t, c.tools, 1)
		assert.Equal(t, "tool", c.tools[0].ID)
	})
}

func TestWithKnowledgeOption(t *testing.T) {
	t.Parallel()
	cfg := &engineknowledge.BaseConfig{ID: "kb"}
	applyAndCheckOption(t, WithKnowledge(cfg), func(c *config) {
		require.Len(t, c.knowledgeBases, 1)
		assert.Equal(t, "kb", c.knowledgeBases[0].ID)
	})
}

func TestWithMemoryOption(t *testing.T) {
	t.Parallel()
	cfg := &enginememory.Config{ID: "mem"}
	applyAndCheckOption(t, WithMemory(cfg), func(c *config) {
		require.Len(t, c.memories, 1)
		assert.Equal(t, "mem", c.memories[0].ID)
	})
}

func TestWithMCPOption(t *testing.T) {
	t.Parallel()
	cfg := &enginemcp.Config{ID: "mcp"}
	applyAndCheckOption(t, WithMCP(cfg), func(c *config) {
		require.Len(t, c.mcps, 1)
		assert.Equal(t, "mcp", c.mcps[0].ID)
	})
}

func TestWithSchemaOption(t *testing.T) {
	t.Parallel()
	value := engineschema.Schema{"type": "object"}
	applyAndCheckOption(t, WithSchema(&value), func(c *config) {
		require.Len(t, c.schemas, 1)
		assert.Equal(t, "object", (*c.schemas[0])["type"])
	})
}

func TestWithModelOption(t *testing.T) {
	t.Parallel()
	cfg := &core.ProviderConfig{Provider: core.ProviderName("openai"), Model: "gpt-4"}
	applyAndCheckOption(t, WithModel(cfg), func(c *config) {
		require.Len(t, c.models, 1)
		assert.Equal(t, core.ProviderName("openai"), c.models[0].Provider)
		assert.Equal(t, "gpt-4", c.models[0].Model)
	})
}

func TestWithScheduleOption(t *testing.T) {
	t.Parallel()
	cfg := &projectschedule.Config{ID: "schedule"}
	applyAndCheckOption(t, WithSchedule(cfg), func(c *config) {
		require.Len(t, c.schedules, 1)
		assert.Equal(t, "schedule", c.schedules[0].ID)
	})
}

func TestWithWebhookOption(t *testing.T) {
	t.Parallel()
	cfg := &enginewebhook.Config{Slug: "webhook"}
	applyAndCheckOption(t, WithWebhook(cfg), func(c *config) {
		require.Len(t, c.webhooks, 1)
		assert.Equal(t, "webhook", c.webhooks[0].Slug)
	})
}

func TestOptionsApplyStandaloneConfigs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		option Option
		check  func(*config)
	}{
		{
			name:   "WithStandaloneTemporal",
			option: WithStandaloneTemporal(&StandaloneTemporalConfig{FrontendPort: 7233}),
			check: func(cfg *config) {
				require.NotNil(t, cfg.standaloneTemporal)
				assert.Equal(t, 7233, cfg.standaloneTemporal.FrontendPort)
			},
		},
		{
			name:   "WithStandaloneRedis",
			option: WithStandaloneRedis(&StandaloneRedisConfig{Port: 6379}),
			check: func(cfg *config) {
				require.NotNil(t, cfg.standaloneRedis)
				assert.Equal(t, 6379, cfg.standaloneRedis.Port)
			},
		},
	}
	for _, tc := range tests {
		caseEntry := tc
		t.Run(caseEntry.name, func(t *testing.T) {
			applyAndCheckOption(t, caseEntry.option, caseEntry.check)
		})
	}
}

func TestWithNilResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		option Option
	}{
		{name: "WithWorkflow", option: WithWorkflow(nil)},
		{name: "WithAgent", option: WithAgent(nil)},
		{name: "WithTool", option: WithTool(nil)},
		{name: "WithKnowledge", option: WithKnowledge(nil)},
		{name: "WithMemory", option: WithMemory(nil)},
		{name: "WithMCP", option: WithMCP(nil)},
		{name: "WithSchema", option: WithSchema(nil)},
		{name: "WithModel", option: WithModel(nil)},
		{name: "WithSchedule", option: WithSchedule(nil)},
		{name: "WithWebhook", option: WithWebhook(nil)},
	}
	for _, tc := range tests {
		caseEntry := tc
		t.Run(caseEntry.name, func(t *testing.T) {
			cfg := defaultConfig()
			caseEntry.option(cfg)
			assert.Zero(t, cfg.resourceCount())
		})
	}
}

func applyAndCheckOption(t *testing.T, option Option, check func(*config)) {
	cfg := defaultConfig()
	require.NotNil(t, cfg)
	option(cfg)
	check(cfg)
}
