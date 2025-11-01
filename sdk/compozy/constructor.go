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
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	engineschema "github.com/compozy/compozy/engine/schema"
	enginetool "github.com/compozy/compozy/engine/tool"
	enginewebhook "github.com/compozy/compozy/engine/webhook"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
)

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
	clones, err := buildResourceClones(cfg)
	if err != nil {
		return nil, err
	}
	engine := &Engine{
		ctx:                ctx,
		mode:               cfg.mode,
		host:               cfg.host,
		port:               cfg.port,
		project:            cfg.project,
		workflows:          clones.workflows,
		agents:             clones.agents,
		tools:              clones.tools,
		knowledgeBases:     clones.knowledgeBases,
		memories:           clones.memories,
		mcps:               clones.mcps,
		schemas:            clones.schemas,
		models:             clones.models,
		schedules:          clones.schedules,
		webhooks:           clones.webhooks,
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

type resourceClones struct {
	workflows      []*engineworkflow.Config
	agents         []*engineagent.Config
	tools          []*enginetool.Config
	knowledgeBases []*engineknowledge.BaseConfig
	memories       []*enginememory.Config
	mcps           []*enginemcp.Config
	schemas        []*engineschema.Schema
	models         []*core.ProviderConfig
	schedules      []*projectschedule.Config
	webhooks       []*enginewebhook.Config
}

func buildResourceClones(cfg *config) (*resourceClones, error) {
	clones := &resourceClones{}
	steps := []func() error{
		func() error {
			workflows, err := cloneWorkflowConfigs(cfg.workflows)
			if err != nil {
				return err
			}
			clones.workflows = workflows
			return nil
		},
		func() error {
			agents, err := cloneAgentConfigs(cfg.agents)
			if err != nil {
				return err
			}
			clones.agents = agents
			return nil
		},
		func() error {
			tools, err := cloneToolConfigs(cfg.tools)
			if err != nil {
				return err
			}
			clones.tools = tools
			return nil
		},
		func() error {
			knowledge, err := cloneKnowledgeConfigs(cfg.knowledgeBases)
			if err != nil {
				return err
			}
			clones.knowledgeBases = knowledge
			return nil
		},
		func() error {
			memories, err := cloneMemoryConfigs(cfg.memories)
			if err != nil {
				return err
			}
			clones.memories = memories
			return nil
		},
		func() error {
			mcps, err := cloneMCPConfigs(cfg.mcps)
			if err != nil {
				return err
			}
			clones.mcps = mcps
			return nil
		},
		func() error {
			schemas, err := cloneSchemaConfigs(cfg.schemas)
			if err != nil {
				return err
			}
			clones.schemas = schemas
			return nil
		},
		func() error {
			models, err := cloneModelConfigs(cfg.models)
			if err != nil {
				return err
			}
			clones.models = models
			return nil
		},
		func() error {
			schedules, err := cloneScheduleConfigs(cfg.schedules)
			if err != nil {
				return err
			}
			clones.schedules = schedules
			return nil
		},
		func() error {
			webhooks, err := cloneWebhookConfigs(cfg.webhooks)
			if err != nil {
				return err
			}
			clones.webhooks = webhooks
			return nil
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return clones, nil
}

func cloneWorkflowConfigs(values []*engineworkflow.Config) ([]*engineworkflow.Config, error) {
	return cloneConfigSlice(values, "workflow")
}

func cloneAgentConfigs(values []*engineagent.Config) ([]*engineagent.Config, error) {
	return cloneConfigSlice(values, "agent")
}

func cloneToolConfigs(values []*enginetool.Config) ([]*enginetool.Config, error) {
	return cloneConfigSlice(values, "tool")
}

func cloneKnowledgeConfigs(values []*engineknowledge.BaseConfig) ([]*engineknowledge.BaseConfig, error) {
	return cloneConfigSlice(values, "knowledge base")
}

func cloneMemoryConfigs(values []*enginememory.Config) ([]*enginememory.Config, error) {
	return cloneConfigSlice(values, "memory")
}

func cloneMCPConfigs(values []*enginemcp.Config) ([]*enginemcp.Config, error) {
	return cloneConfigSlice(values, "mcp")
}

func cloneSchemaConfigs(values []*engineschema.Schema) ([]*engineschema.Schema, error) {
	return cloneConfigSlice(values, "schema")
}

func cloneModelConfigs(values []*core.ProviderConfig) ([]*core.ProviderConfig, error) {
	return cloneConfigSlice(values, "model")
}

func cloneScheduleConfigs(values []*projectschedule.Config) ([]*projectschedule.Config, error) {
	return cloneConfigSlice(values, "schedule")
}

func cloneWebhookConfigs(values []*enginewebhook.Config) ([]*enginewebhook.Config, error) {
	return cloneConfigSlice(values, "webhook")
}

func cloneConfigSlice[T any](values []*T, label string) ([]*T, error) {
	if len(values) == 0 {
		return make([]*T, 0), nil
	}
	cloned := make([]*T, 0, len(values))
	for _, value := range values {
		if value == nil {
			cloned = append(cloned, nil)
			continue
		}
		clonedValue, err := core.DeepCopy(value)
		if err != nil {
			return nil, fmt.Errorf("clone %s configs: %w", label, err)
		}
		cloned = append(cloned, clonedValue)
	}
	return cloned, nil
}
