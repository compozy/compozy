package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/frontmatter"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/goccy/go-yaml"
)

// EditAgentDefFile rewrites one AGENT.md frontmatter block while preserving the prompt body.
func EditAgentDefFile(path string, mutate func(*AgentDef) error) (AgentDef, error) {
	if strings.TrimSpace(path) == "" {
		return AgentDef{}, fmt.Errorf("config: agent file path is required")
	}
	if mutate == nil {
		return AgentDef{}, fmt.Errorf("config: agent mutate callback is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return AgentDef{}, fmt.Errorf("read agent file %q: %w", path, err)
	}
	parts, err := frontmatter.Split(content)
	if err != nil {
		return AgentDef{}, fmt.Errorf("parse agent file %q: %w", path, wrapFrontmatterError(err))
	}

	var parsed parsedAgentDef
	if err := decodeAgentFrontmatter(parts.Metadata, &parsed); err != nil {
		return AgentDef{}, fmt.Errorf("parse agent file %q: %w", path, err)
	}

	agent, err := agentDefFromParsedFile(path, parts, parsed)
	if err != nil {
		return AgentDef{}, err
	}
	originalAgentName := agent.Name

	if err := mutate(&agent); err != nil {
		return AgentDef{}, err
	}
	agent.Name = NormalizeAgentName(agent.Name)
	agent.Skills = normalizeAgentSkillsConfig(agent.Skills)
	agent.CategoryPath = normalizeAgentCategoryPath(agent.CategoryPath)
	agent.Hooks, err = normalizeAgentDefinitionHookEdits(agent.Hooks, originalAgentName, agent.Name)
	if err != nil {
		return AgentDef{}, fmt.Errorf("validate agent file %q: %w", path, err)
	}
	if err := agent.Validate(); err != nil {
		return AgentDef{}, fmt.Errorf("validate agent file %q: %w", path, err)
	}

	if err := applyAgentDefToParsed(&parsed, agent); err != nil {
		return AgentDef{}, fmt.Errorf("marshal agent file %q: %w", path, err)
	}
	meta, err := yaml.Marshal(parsed)
	if err != nil {
		return AgentDef{}, fmt.Errorf("marshal agent file %q: %w", path, err)
	}

	rendered := renderAgentMarkdown(meta, agent.Prompt)
	if err := writePersistedFile(path, rendered); err != nil {
		return AgentDef{}, fmt.Errorf("write agent file %q: %w", path, err)
	}

	agent.SourcePath = filepath.Clean(path)
	return agent, nil
}

func agentDefFromParsedFile(
	path string,
	parts frontmatter.Parts,
	parsed parsedAgentDef,
) (AgentDef, error) {
	agent := AgentDef{
		Name:            NormalizeAgentName(parsed.Name),
		Provider:        strings.TrimSpace(parsed.Provider),
		Command:         strings.TrimSpace(parsed.Command),
		Model:           strings.TrimSpace(parsed.Model),
		ReasoningEffort: strings.TrimSpace(parsed.ReasoningEffort),
		Tools:           normalizeAgentToolPatterns(parsed.Tools),
		Toolsets:        normalizeAgentToolsetRefs(parsed.Toolsets),
		DenyTools:       normalizeAgentToolPatterns(parsed.DenyTools),
		Permissions:     strings.TrimSpace(parsed.Permissions),
		Skills:          normalizeAgentSkillsConfig(parsed.Skills),
		CategoryPath:    normalizeAgentCategoryPath(parsed.CategoryPath),
		MCPServers:      cloneMCPServers(parsed.MCPServers),
		Prompt:          strings.TrimSpace(parts.Body),
		SourcePath:      filepath.Clean(path),
	}
	if len(parsed.Hooks) == 0 {
		return agent, nil
	}
	agent.Hooks = make([]hookspkg.HookDecl, 0, len(parsed.Hooks))
	for idx := range parsed.Hooks {
		raw := &parsed.Hooks[idx]
		decl, convErr := raw.toHookDecl(hookspkg.HookSourceAgentDefinition, agent.Name)
		if convErr != nil {
			return AgentDef{}, fmt.Errorf("parse agent file %q hook %d: %w", path, idx, convErr)
		}
		agent.Hooks = append(agent.Hooks, decl)
	}
	return agent, nil
}

func applyAgentDefToParsed(parsed *parsedAgentDef, agent AgentDef) error {
	parsed.Name = agent.Name
	parsed.Provider = strings.TrimSpace(agent.Provider)
	parsed.Command = strings.TrimSpace(agent.Command)
	parsed.Model = strings.TrimSpace(agent.Model)
	parsed.ReasoningEffort = strings.TrimSpace(agent.ReasoningEffort)
	parsed.Tools = cloneStrings(agent.Tools)
	parsed.Toolsets = cloneStrings(agent.Toolsets)
	parsed.DenyTools = cloneStrings(agent.DenyTools)
	parsed.Permissions = strings.TrimSpace(agent.Permissions)
	parsed.Skills = normalizeAgentSkillsConfig(agent.Skills)
	parsed.CategoryPath = normalizeAgentCategoryPath(agent.CategoryPath)
	parsed.MCPServers = cloneMCPServers(agent.MCPServers)
	hooks, err := parsedHookDeclarationsFromHookDecls(agent.Hooks, agent.Name)
	if err != nil {
		return err
	}
	parsed.Hooks = hooks
	return nil
}

func normalizeAgentDefinitionHookEdits(
	hooks []hookspkg.HookDecl,
	originalAgentName string,
	agentName string,
) ([]hookspkg.HookDecl, error) {
	if len(hooks) == 0 {
		return nil, nil
	}

	normalized := make([]hookspkg.HookDecl, 0, len(hooks))
	for idx, hook := range hooks {
		decl := cloneHookDecl(hook)
		decl.Source = hookspkg.HookSourceAgentDefinition
		matcherAgent := strings.TrimSpace(decl.Matcher.AgentName)
		switch matcherAgent {
		case "", originalAgentName, agentName:
			decl.Matcher.AgentName = agentName
		default:
			return nil, fmt.Errorf("agent.hooks[%d]: matcher.agent_name must match agent name %q", idx, agentName)
		}
		normalized = append(normalized, decl)
	}
	return normalized, nil
}

func renderAgentMarkdown(meta []byte, prompt string) []byte {
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(meta)
	if builder.Len() == 0 || !strings.HasSuffix(builder.String(), "\n") {
		builder.WriteByte('\n')
	}
	builder.WriteString("---\n")
	if strings.TrimSpace(prompt) != "" {
		builder.WriteByte('\n')
		builder.WriteString(strings.TrimSpace(prompt))
		builder.WriteByte('\n')
	}
	return []byte(builder.String())
}
