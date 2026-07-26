package config

import (
	"errors"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/reasoning"
)

// Validate ensures the parsed agent definition is usable.
func (a AgentDef) Validate() error {
	switch {
	case NormalizeAgentName(a.Name) == "":
		return errors.New("agent name is required")
	case strings.TrimSpace(a.Prompt) == "":
		return errors.New("agent prompt is required")
	}
	if err := ValidateAgentName(a.Name); err != nil {
		return err
	}
	if effort := strings.TrimSpace(a.ReasoningEffort); a.ReasoningEffort != "" {
		if effort != a.ReasoningEffort || !reasoning.IsValid(effort) {
			return &reasoning.InvalidEffortError{Path: "agent.reasoning_effort", Value: a.ReasoningEffort}
		}
	}
	if strings.TrimSpace(a.Permissions) != "" {
		if err := PermissionMode(a.Permissions).Validate("agent.permissions"); err != nil {
			return err
		}
	}
	if err := validateAgentToolPatterns(a.Tools, "agent.tools"); err != nil {
		return err
	}
	if err := validateAgentToolsets(a.Toolsets, "agent.toolsets"); err != nil {
		return err
	}
	if err := validateAgentToolPatterns(a.DenyTools, "agent.deny_tools"); err != nil {
		return err
	}
	if err := validateAgentCategoryPath(a.CategoryPath); err != nil {
		return err
	}

	for i, server := range a.MCPServers {
		if err := server.Validate(fmt.Sprintf("agent.mcp_servers[%d]", i)); err != nil {
			return err
		}
	}
	for i, hook := range a.Hooks {
		if err := hookspkg.ValidateHookDecl(hook); err != nil {
			return fmt.Errorf("agent.hooks[%d]: %w", i, err)
		}
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
