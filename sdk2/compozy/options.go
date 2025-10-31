package compozy

import (
	"strings"

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

// Option configures the Compozy engine during construction.
type Option func(*config)

const (
	defaultMode = ModeStandalone
	defaultHost = "127.0.0.1"
)

type config struct {
	mode               Mode
	host               string
	port               int
	project            *engineproject.Config
	workflows          []*engineworkflow.Config
	agents             []*engineagent.Config
	tools              []*enginetool.Config
	knowledgeBases     []*engineknowledge.BaseConfig
	memories           []*enginememory.Config
	mcps               []*enginemcp.Config
	schemas            []*engineschema.Schema
	models             []*core.ProviderConfig
	schedules          []*projectschedule.Config
	webhooks           []*enginewebhook.Config
	standaloneTemporal *StandaloneTemporalConfig
	standaloneRedis    *StandaloneRedisConfig
}

func defaultConfig() *config {
	return &config{
		mode:           defaultMode,
		host:           defaultHost,
		workflows:      make([]*engineworkflow.Config, 0),
		agents:         make([]*engineagent.Config, 0),
		tools:          make([]*enginetool.Config, 0),
		knowledgeBases: make([]*engineknowledge.BaseConfig, 0),
		memories:       make([]*enginememory.Config, 0),
		mcps:           make([]*enginemcp.Config, 0),
		schemas:        make([]*engineschema.Schema, 0),
		models:         make([]*core.ProviderConfig, 0),
		schedules:      make([]*projectschedule.Config, 0),
		webhooks:       make([]*enginewebhook.Config, 0),
	}
}

// WithMode sets the deployment mode for the engine.
func WithMode(mode Mode) Option {
	return func(c *config) {
		if c == nil {
			return
		}
		c.mode = mode
	}
}

// WithHost overrides the bind host for the embedded server.
func WithHost(host string) Option {
	return func(c *config) {
		if c == nil {
			return
		}
		c.host = strings.TrimSpace(host)
	}
}

// WithPort sets the HTTP port for the embedded server.
func WithPort(port int) Option {
	return func(c *config) {
		if c == nil {
			return
		}
		c.port = port
	}
}

// WithProject registers the project configuration used for resource scoping.
func WithProject(projectCfg *engineproject.Config) Option {
	return func(c *config) {
		if c == nil || projectCfg == nil {
			return
		}
		c.project = projectCfg
	}
}

// WithWorkflow queues a workflow configuration for registration.
func WithWorkflow(workflowCfg *engineworkflow.Config) Option {
	return func(c *config) {
		if c == nil || workflowCfg == nil {
			return
		}
		c.workflows = append(c.workflows, workflowCfg)
	}
}

// WithAgent queues an agent configuration for registration.
func WithAgent(agentCfg *engineagent.Config) Option {
	return func(c *config) {
		if c == nil || agentCfg == nil {
			return
		}
		c.agents = append(c.agents, agentCfg)
	}
}

// WithTool queues a tool configuration for registration.
func WithTool(toolCfg *enginetool.Config) Option {
	return func(c *config) {
		if c == nil || toolCfg == nil {
			return
		}
		c.tools = append(c.tools, toolCfg)
	}
}

// WithKnowledge queues a knowledge base configuration for registration.
func WithKnowledge(knowledgeCfg *engineknowledge.BaseConfig) Option {
	return func(c *config) {
		if c == nil || knowledgeCfg == nil {
			return
		}
		c.knowledgeBases = append(c.knowledgeBases, knowledgeCfg)
	}
}

// WithMemory queues a memory configuration for registration.
func WithMemory(memoryCfg *enginememory.Config) Option {
	return func(c *config) {
		if c == nil || memoryCfg == nil {
			return
		}
		c.memories = append(c.memories, memoryCfg)
	}
}

// WithMCP queues an MCP configuration for registration.
func WithMCP(mcpCfg *enginemcp.Config) Option {
	return func(c *config) {
		if c == nil || mcpCfg == nil {
			return
		}
		c.mcps = append(c.mcps, mcpCfg)
	}
}

// WithSchema queues a schema configuration for registration.
func WithSchema(schemaCfg *engineschema.Schema) Option {
	return func(c *config) {
		if c == nil || schemaCfg == nil {
			return
		}
		c.schemas = append(c.schemas, schemaCfg)
	}
}

// WithModel queues a model provider configuration for registration.
func WithModel(modelCfg *core.ProviderConfig) Option {
	return func(c *config) {
		if c == nil || modelCfg == nil {
			return
		}
		c.models = append(c.models, modelCfg)
	}
}

// WithSchedule queues a schedule configuration for registration.
func WithSchedule(scheduleCfg *projectschedule.Config) Option {
	return func(c *config) {
		if c == nil || scheduleCfg == nil {
			return
		}
		c.schedules = append(c.schedules, scheduleCfg)
	}
}

// WithWebhook queues a webhook configuration for registration.
func WithWebhook(webhookCfg *enginewebhook.Config) Option {
	return func(c *config) {
		if c == nil || webhookCfg == nil {
			return
		}
		c.webhooks = append(c.webhooks, webhookCfg)
	}
}

// WithStandaloneTemporal configures the embedded Temporal server for standalone mode.
func WithStandaloneTemporal(cfg *StandaloneTemporalConfig) Option {
	return func(c *config) {
		if c == nil {
			return
		}
		c.standaloneTemporal = cfg
	}
}

// WithStandaloneRedis configures the embedded Redis server for standalone mode.
func WithStandaloneRedis(cfg *StandaloneRedisConfig) Option {
	return func(c *config) {
		if c == nil {
			return
		}
		c.standaloneRedis = cfg
	}
}
