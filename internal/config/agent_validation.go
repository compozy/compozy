package config

import (
	"errors"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/reasoning"
)

// Validate ensures the parsed agent definition is usable.
func (a AgentDef) Validate() error {
	if err := a.ValidateIdentity(); err != nil {
		return err
	}
	if a.WorkspaceAccess != "" && a.WorkspaceAccess != "unrestricted" {
		return fmt.Errorf("agent.workspace_access must be absent or unrestricted")
	}
	if err := a.SessionEnv.Validate("agent.session_env"); err != nil {
		return err
	}
	switch {
	case NormalizeAgentName(a.Name) == "":
		return errors.New("agent name is required")
	case strings.TrimSpace(a.Prompt) == "":
		return errors.New("agent prompt is required")
	}
	if err := ValidateAgentName(a.Name); err != nil {
		return err
	}
	if err := validateAgentRuntimeDefaults(a); err != nil {
		return err
	}
	if strings.TrimSpace(a.Permissions) != "" {
		if err := PermissionMode(a.Permissions).Validate("agent.permissions"); err != nil {
			return err
		}
	}
	if err := validateAgentToolPolicy(a.Tools, a.Toolsets, a.DenyTools); err != nil {
		return err
	}
	if err := validateAgentCategoryPath(a.CategoryPath); err != nil {
		return err
	}

	if err := validateFallbackChain(AgentFallbackChainPath(a.Name), a.FallbackChain, nil, false); err != nil {
		return err
	}
	if err := validateAgentMCPServersAndHooks(a.MCPServers, a.Hooks); err != nil {
		return err
	}
	normalizedCapabilities, err := normalizeCapabilityCatalog(a.Capabilities, "agent.capabilities")
	if err != nil {
		return err
	}
	if a.Capabilities != nil {
		*a.Capabilities = *normalizedCapabilities
	}

	return nil
}

func validateAgentToolPolicy(tools, toolsets, denyTools []string) error {
	if err := validateAgentToolPatterns(tools, "agent.tools"); err != nil {
		return err
	}
	if err := validateAgentToolsets(toolsets, "agent.toolsets"); err != nil {
		return err
	}
	return validateAgentToolPatterns(denyTools, "agent.deny_tools")
}

func validateAgentMCPServersAndHooks(servers []MCPServer, hooks []hookspkg.HookDecl) error {
	for i, server := range servers {
		if err := server.Validate(fmt.Sprintf("agent.mcp_servers[%d]", i)); err != nil {
			return err
		}
	}
	for i, hook := range hooks {
		if err := hookspkg.ValidateHookDecl(hook); err != nil {
			return fmt.Errorf("agent.hooks[%d]: %w", i, err)
		}
	}
	return nil
}

func validateAgentRuntimeDefaults(agent AgentDef) error {
	if effort := strings.TrimSpace(agent.ReasoningEffort); agent.ReasoningEffort != "" {
		if effort != agent.ReasoningEffort || !reasoning.IsValid(effort) {
			return &reasoning.InvalidEffortError{Path: "agent.reasoning_effort", Value: agent.ReasoningEffort}
		}
	}
	if err := validateAgentSpeed(agent.SpeedValue(), "agent.speed"); err != nil {
		return err
	}
	return validateACPOptionSelections("agent.acp_options", agent.ACPOptionsValue())
}

func hasEmbeddedTabFrontmatterKey(data []byte) bool {
	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.Contains(key, "\t") {
			return true
		}
	}
	return false
}
