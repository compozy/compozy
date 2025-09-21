package main

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

type captureTarget int

type schemaPostProcessor func(map[string]any) bool

type schemaDefinition struct {
	name        string
	title       string
	source      any
	capture     captureTarget
	postProcess schemaPostProcessor
}

const (
	captureNone captureTarget = iota
	captureProjectRuntime
	captureConfigRuntime
)

func (d schemaDefinition) fileName() string {
	return d.name + ".json"
}

var schemaDefinitions = []schemaDefinition{
	{
		name:        "agent",
		title:       "Agent Configuration",
		source:      &agent.Config{},
		postProcess: postProcessAgentSchema,
	},
	{
		name:        "action-config",
		title:       "Agent Action Configuration",
		source:      &agent.ActionConfig{},
		postProcess: postProcessActionConfigSchema,
	},
	{
		name:   "author",
		title:  "Author Configuration",
		source: &core.Author{},
	},
	{
		name:        "project",
		title:       "Project Configuration",
		source:      &project.Config{},
		postProcess: postProcessProjectSchema,
	},
	{
		name:   "project-options",
		title:  "Project Options",
		source: &project.Opts{},
	},
	{
		name:   "provider",
		title:  "Provider Configuration",
		source: &core.ProviderConfig{},
	},
	{
		name:    "runtime",
		title:   "Compozy Runtime Configuration",
		source:  &project.RuntimeConfig{},
		capture: captureProjectRuntime,
	},
	{
		name:   "mcp",
		title:  "MCP Configuration",
		source: &mcp.Config{},
	},
	{
		name:   "memory",
		title:  "Memory Configuration",
		source: &memory.Config{},
	},
	{
		name:        "task",
		title:       "Task Configuration",
		source:      &task.Config{},
		postProcess: postProcessTaskSchema,
	},
	{
		name:   "tool",
		title:  "Tool Configuration",
		source: &tool.Config{},
	},
	{
		name:        "workflow",
		title:       "Workflow Configuration",
		source:      &workflow.Config{},
		postProcess: postProcessWorkflowSchema,
	},
	{
		name:   "webhook",
		title:  "Webhook Configuration",
		source: &webhook.Config{},
	},
	{
		name:   "cache",
		title:  "Cache Configuration",
		source: &cache.Config{},
	},
	{
		name:   "autoload",
		title:  "Autoload Configuration",
		source: &autoload.Config{},
	},
	{
		name:   "monitoring",
		title:  "Monitoring Configuration",
		source: &monitoring.Config{},
	},
	{
		name:   "config",
		title:  "Application Configuration",
		source: &config.Config{},
	},
	{
		name:   "config-server",
		title:  "Server Configuration",
		source: &config.ServerConfig{},
	},
	{
		name:   "config-database",
		title:  "Database Configuration",
		source: &config.DatabaseConfig{},
	},
	{
		name:   "config-temporal",
		title:  "Temporal Configuration",
		source: &config.TemporalConfig{},
	},
	{
		name:    "config-runtime",
		title:   "System Runtime Configuration",
		source:  &config.RuntimeConfig{},
		capture: captureConfigRuntime,
	},
	{
		name:   "config-limits",
		title:  "Limits Configuration",
		source: &config.LimitsConfig{},
	},
	{
		name:   "config-memory",
		title:  "Memory Configuration",
		source: &config.MemoryConfig{},
	},
	{
		name:   "config-llm",
		title:  "LLM Configuration",
		source: &config.LLMConfig{},
	},
	{
		name:   "config-ratelimit",
		title:  "Rate Limit Configuration",
		source: &config.RateLimitConfig{},
	},
	{
		name:   "config-cli",
		title:  "CLI Configuration",
		source: &config.CLIConfig{},
	},
	{
		name:   "config-redis",
		title:  "Redis Configuration",
		source: &config.RedisConfig{},
	},
	{
		name:   "config-cache",
		title:  "Cache Configuration",
		source: &config.CacheConfig{},
	},
	{
		name:   "config-worker",
		title:  "Worker Configuration",
		source: &config.WorkerConfig{},
	},
	{
		name:   "config-mcpproxy",
		title:  "MCP Proxy Configuration",
		source: &config.MCPProxyConfig{},
	},
	{
		name:   "config-attachments",
		title:  "Attachments Configuration",
		source: &config.AttachmentsConfig{},
	},
	{
		name:   "config-webhooks",
		title:  "Webhooks Configuration",
		source: &config.WebhooksConfig{},
	},
}
