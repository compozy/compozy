package compozy

import (
	"context"
	"fmt"
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

// Engine represents an instantiated Compozy SDK core.
type Engine struct {
	ctx                context.Context
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
	started            bool
}

// New constructs an Engine using the provided functional options.
func New(ctx context.Context, opts ...Option) (*Engine, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(cfg)
	}
	cfg.normalize()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	engine := &Engine{
		ctx:                ctx,
		mode:               cfg.mode,
		host:               cfg.host,
		port:               cfg.port,
		project:            cfg.project,
		workflows:          cloneWorkflowConfigs(cfg.workflows),
		agents:             cloneAgentConfigs(cfg.agents),
		tools:              cloneToolConfigs(cfg.tools),
		knowledgeBases:     cloneKnowledgeConfigs(cfg.knowledgeBases),
		memories:           cloneMemoryConfigs(cfg.memories),
		mcps:               cloneMCPConfigs(cfg.mcps),
		schemas:            cloneSchemaConfigs(cfg.schemas),
		models:             cloneModelConfigs(cfg.models),
		schedules:          cloneScheduleConfigs(cfg.schedules),
		webhooks:           cloneWebhookConfigs(cfg.webhooks),
		standaloneTemporal: cfg.standaloneTemporal,
		standaloneRedis:    cfg.standaloneRedis,
	}
	return engine, nil
}

func (c *config) normalize() {
	c.host = strings.TrimSpace(c.host)
	if c.host == "" {
		c.host = defaultHost
	}
}

func (c *config) validate() error {
	if c.resourceCount() == 0 {
		return fmt.Errorf("at least one resource must be registered")
	}
	return nil
}

func (c *config) resourceCount() int {
	count := 0
	if c.project != nil {
		count++
	}
	count += len(c.workflows)
	count += len(c.agents)
	count += len(c.tools)
	count += len(c.knowledgeBases)
	count += len(c.memories)
	count += len(c.mcps)
	count += len(c.schemas)
	count += len(c.models)
	count += len(c.schedules)
	count += len(c.webhooks)
	return count
}

func cloneWorkflowConfigs(values []*engineworkflow.Config) []*engineworkflow.Config {
	if len(values) == 0 {
		return make([]*engineworkflow.Config, 0)
	}
	return append(make([]*engineworkflow.Config, 0, len(values)), values...)
}

func cloneAgentConfigs(values []*engineagent.Config) []*engineagent.Config {
	if len(values) == 0 {
		return make([]*engineagent.Config, 0)
	}
	return append(make([]*engineagent.Config, 0, len(values)), values...)
}

func cloneToolConfigs(values []*enginetool.Config) []*enginetool.Config {
	if len(values) == 0 {
		return make([]*enginetool.Config, 0)
	}
	return append(make([]*enginetool.Config, 0, len(values)), values...)
}

func cloneKnowledgeConfigs(values []*engineknowledge.BaseConfig) []*engineknowledge.BaseConfig {
	if len(values) == 0 {
		return make([]*engineknowledge.BaseConfig, 0)
	}
	return append(make([]*engineknowledge.BaseConfig, 0, len(values)), values...)
}

func cloneMemoryConfigs(values []*enginememory.Config) []*enginememory.Config {
	if len(values) == 0 {
		return make([]*enginememory.Config, 0)
	}
	return append(make([]*enginememory.Config, 0, len(values)), values...)
}

func cloneMCPConfigs(values []*enginemcp.Config) []*enginemcp.Config {
	if len(values) == 0 {
		return make([]*enginemcp.Config, 0)
	}
	return append(make([]*enginemcp.Config, 0, len(values)), values...)
}

func cloneSchemaConfigs(values []*engineschema.Schema) []*engineschema.Schema {
	if len(values) == 0 {
		return make([]*engineschema.Schema, 0)
	}
	return append(make([]*engineschema.Schema, 0, len(values)), values...)
}

func cloneModelConfigs(values []*core.ProviderConfig) []*core.ProviderConfig {
	if len(values) == 0 {
		return make([]*core.ProviderConfig, 0)
	}
	return append(make([]*core.ProviderConfig, 0, len(values)), values...)
}

func cloneScheduleConfigs(values []*projectschedule.Config) []*projectschedule.Config {
	if len(values) == 0 {
		return make([]*projectschedule.Config, 0)
	}
	return append(make([]*projectschedule.Config, 0, len(values)), values...)
}

func cloneWebhookConfigs(values []*enginewebhook.Config) []*enginewebhook.Config {
	if len(values) == 0 {
		return make([]*enginewebhook.Config, 0)
	}
	return append(make([]*enginewebhook.Config, 0, len(values)), values...)
}
