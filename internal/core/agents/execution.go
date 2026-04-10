package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

var errRuntimeConfigNil = errors.New("runtime config is nil")

var promptMetadataEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\r", "\\r",
	"\n", "\\n",
)

// ExecutionContext captures the resolved reusable-agent inputs needed by the
// execution pipeline after runtime precedence has been applied.
type ExecutionContext struct {
	Agent       ResolvedAgent
	Catalog     Catalog
	BaseRuntime NestedBaseRuntime
}

// ResolveExecutionContext resolves the selected reusable agent, applies its
// runtime defaults using the documented precedence rules, and returns the
// reusable prompt-assembly context. When no agent is selected, it returns nil.
func ResolveExecutionContext(ctx context.Context, cfg *model.RuntimeConfig) (*ExecutionContext, error) {
	return resolveExecutionContext(ctx, New(), cfg)
}

func resolveExecutionContext(
	ctx context.Context,
	registry *Registry,
	cfg *model.RuntimeConfig,
) (*ExecutionContext, error) {
	if cfg == nil {
		return nil, errRuntimeConfigNil
	}
	cfg.ApplyDefaults()

	agentName := strings.TrimSpace(cfg.AgentName)
	if agentName == "" {
		return nil, nil
	}
	if registry == nil {
		registry = New()
	}

	baseRuntime := captureNestedBaseRuntime(cfg)
	catalog, err := registry.Discover(ctx, cfg.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("discover reusable agents: %w", err)
	}

	resolved, err := catalog.Resolve(agentName)
	if err != nil {
		return nil, fmt.Errorf("resolve reusable agent %q: %w", agentName, err)
	}

	applyRuntimePrecedence(cfg, resolved.Runtime)
	cfg.AgentName = resolved.Name

	return &ExecutionContext{
		Agent:       resolved,
		Catalog:     catalog,
		BaseRuntime: baseRuntime,
	}, nil
}

// SystemPrompt assembles the canonical system prompt for a selected reusable
// agent. When no execution context is present, it preserves the existing base
// system prompt unchanged.
func (c *ExecutionContext) SystemPrompt(baseSystemPrompt string) string {
	if c == nil {
		return baseSystemPrompt
	}

	sections := make([]string, 0, 4)
	if trimmed := strings.TrimSpace(baseSystemPrompt); trimmed != "" {
		sections = append(sections, trimmed)
	}
	sections = append(
		sections,
		buildAgentMetadataBlock(c.Agent),
		buildAvailableAgentsBlock(c.Agent.Name, c.Catalog.Agents),
	)
	if body := strings.TrimSpace(c.Agent.Prompt); body != "" {
		sections = append(sections, body)
	}
	return strings.Join(sections, "\n\n")
}

func applyRuntimePrecedence(cfg *model.RuntimeConfig, defaults RuntimeDefaults) {
	if cfg == nil {
		return
	}
	if !cfg.ExplicitRuntime.IDE && strings.TrimSpace(defaults.IDE) != "" {
		cfg.IDE = defaults.IDE
	}
	if !cfg.ExplicitRuntime.Model && strings.TrimSpace(defaults.Model) != "" {
		cfg.Model = defaults.Model
	}
	if !cfg.ExplicitRuntime.ReasoningEffort && strings.TrimSpace(defaults.ReasoningEffort) != "" {
		cfg.ReasoningEffort = defaults.ReasoningEffort
	}
	if !cfg.ExplicitRuntime.AccessMode && strings.TrimSpace(defaults.AccessMode) != "" {
		cfg.AccessMode = defaults.AccessMode
	}
}

func buildAgentMetadataBlock(agent ResolvedAgent) string {
	lines := []string{
		"<agent_metadata>",
		"name: " + escapePromptMetadataValue(agent.Name),
		"title: " + escapePromptMetadataValue(agent.Metadata.Title),
		"description: " + escapePromptMetadataValue(agent.Metadata.Description),
		"source: " + string(agent.Source.Scope),
		"</agent_metadata>",
	}
	return strings.Join(lines, "\n")
}

func buildAvailableAgentsBlock(selectedName string, agents []ResolvedAgent) string {
	lines := []string{"<available_agents>"}
	for idx := range agents {
		if agents[idx].Name == selectedName {
			continue
		}
		lines = append(lines, formatDiscoveryCatalogEntry(&agents[idx]))
	}
	lines = append(lines, "</available_agents>")
	return strings.Join(lines, "\n")
}

func formatDiscoveryCatalogEntry(agent *ResolvedAgent) string {
	var entry strings.Builder
	entry.WriteString("- ")
	entry.WriteString(escapePromptMetadataValue(agent.Name))
	entry.WriteString(":")
	if description := strings.TrimSpace(agent.Metadata.Description); description != "" {
		entry.WriteString(" ")
		entry.WriteString(escapePromptMetadataValue(description))
	}
	entry.WriteString(" (")
	entry.WriteString(string(agent.Source.Scope))
	entry.WriteString(")")
	return entry.String()
}

func escapePromptMetadataValue(value string) string {
	return promptMetadataEscaper.Replace(strings.TrimSpace(value))
}
