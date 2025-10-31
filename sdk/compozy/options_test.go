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

func TestOptionsApplyPrimaryCollections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		option Option
		check  func(*config)
	}{
		{
			name: "WithWorkflow",
			option: func() Option {
				cfg := &engineworkflow.Config{ID: "wf"}
				return WithWorkflow(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.workflows, 1)
				assert.Equal(t, "wf", cfg.workflows[0].ID)
			},
		},
		{
			name: "WithAgent",
			option: func() Option {
				cfg := &engineagent.Config{ID: "agent"}
				return WithAgent(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.agents, 1)
				assert.Equal(t, "agent", cfg.agents[0].ID)
			},
		},
		{
			name: "WithTool",
			option: func() Option {
				cfg := &enginetool.Config{ID: "tool"}
				return WithTool(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.tools, 1)
				assert.Equal(t, "tool", cfg.tools[0].ID)
			},
		},
		{
			name: "WithKnowledge",
			option: func() Option {
				cfg := &engineknowledge.BaseConfig{ID: "kb"}
				return WithKnowledge(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.knowledgeBases, 1)
				assert.Equal(t, "kb", cfg.knowledgeBases[0].ID)
			},
		},
		{
			name: "WithMemory",
			option: func() Option {
				cfg := &enginememory.Config{ID: "mem"}
				return WithMemory(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.memories, 1)
				assert.Equal(t, "mem", cfg.memories[0].ID)
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

func TestOptionsApplySecondaryCollections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		option Option
		check  func(*config)
	}{
		{
			name: "WithMCP",
			option: func() Option {
				cfg := &enginemcp.Config{ID: "mcp"}
				return WithMCP(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.mcps, 1)
				assert.Equal(t, "mcp", cfg.mcps[0].ID)
			},
		},
		{
			name: "WithSchema",
			option: func() Option {
				value := engineschema.Schema{"type": "object"}
				return WithSchema(&value)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.schemas, 1)
				assert.Equal(t, "object", (*cfg.schemas[0])["type"])
			},
		},
		{
			name: "WithModel",
			option: func() Option {
				cfg := &core.ProviderConfig{Provider: core.ProviderName("openai"), Model: "gpt-4"}
				return WithModel(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.models, 1)
				assert.Equal(t, core.ProviderName("openai"), cfg.models[0].Provider)
				assert.Equal(t, "gpt-4", cfg.models[0].Model)
			},
		},
		{
			name: "WithSchedule",
			option: func() Option {
				cfg := &projectschedule.Config{ID: "schedule"}
				return WithSchedule(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.schedules, 1)
				assert.Equal(t, "schedule", cfg.schedules[0].ID)
			},
		},
		{
			name: "WithWebhook",
			option: func() Option {
				cfg := &enginewebhook.Config{Slug: "webhook"}
				return WithWebhook(cfg)
			}(),
			check: func(cfg *config) {
				require.Len(t, cfg.webhooks, 1)
				assert.Equal(t, "webhook", cfg.webhooks[0].Slug)
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
