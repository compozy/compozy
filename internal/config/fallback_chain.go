package config

import (
	"fmt"
	"strings"
)

// validateFallbackChain validates ordered fallback routes under chainPath (for example
// `roles.auto_title.fallback_chain` or `agent "reviewer" fallback_chain`). Parsing is the
// only command validation: there is no route-count or command-length limit, so every
// chain that loaded before `command` existed keeps loading. When requireResolver is false
// and resolver is nil, provider availability is not checked (static AGENT.md validation).
func validateFallbackChain(
	chainPath string,
	fallbacks []RoleFallback,
	resolver providerResolver,
	requireResolver bool,
) error {
	for i, fallback := range fallbacks {
		fallbackPath := fmt.Sprintf("%s[%d]", chainPath, i)
		if err := validateFallbackRoute(fallbackPath, fallback); err != nil {
			return err
		}
		if resolver == nil {
			if requireResolver {
				return fmt.Errorf("%s.provider resolver is required", fallbackPath)
			}
			continue
		}
		if _, err := resolver.ResolveProvider(strings.TrimSpace(fallback.Provider)); err != nil {
			return fmt.Errorf("%s.provider: %w", fallbackPath, err)
		}
	}
	return nil
}

func validateFallbackRoute(fallbackPath string, fallback RoleFallback) error {
	if strings.TrimSpace(fallback.Provider) == "" {
		return fmt.Errorf("%s.provider is required", fallbackPath)
	}
	if strings.TrimSpace(fallback.Model) == "" {
		return fmt.Errorf("%s.model is required", fallbackPath)
	}
	if err := validateRoleReasoningEffort(fallbackPath+".reasoning_effort", fallback.ReasoningEffort); err != nil {
		return err
	}
	if err := validateAgentSpeed(fallback.Speed, fallbackPath+".speed"); err != nil {
		return err
	}
	if err := validateRoleACPOptions(fallbackPath+".acp_options", fallback.ACPOptions); err != nil {
		return err
	}
	if err := validateRoleACPOptionConflicts(
		fallbackPath+".acp_options",
		fallback.ACPOptions,
		fallback.Speed,
		fallback.ReasoningEffort,
	); err != nil {
		return err
	}
	return ValidateFallbackRouteCommand(fallbackPath, fallback.Command)
}

// ValidateFallbackRouteCommand validates one route command with the provider-auth parser.
// Whitespace-only commands mean "inherit" and are accepted.
func ValidateFallbackRouteCommand(fallbackPath, command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if _, err := ParseLaunchCommand(command); err != nil {
		return fmt.Errorf("%s.command: %w", fallbackPath, err)
	}
	return nil
}

// AgentFallbackChainPath is the validation path prefix for one agent's fallback chain.
func AgentFallbackChainPath(agentName string) string {
	return fmt.Sprintf("agent %q fallback_chain", NormalizeAgentName(agentName))
}

// ValidateAgentFallbackChain validates an agent chain, including provider availability
// against this configuration. It is the authoring-surface check (HTTP/UDS, CLI, tools).
func (c *Config) ValidateAgentFallbackChain(agent AgentDef) error {
	if len(agent.FallbackChain) == 0 {
		return nil
	}
	return validateFallbackChain(AgentFallbackChainPath(agent.Name), agent.FallbackChain, c, true)
}

// CloneRoleFallbacks deep-copies an ordered fallback chain.
func CloneRoleFallbacks(source []RoleFallback) []RoleFallback {
	return cloneRoleFallbacks(source)
}

func normalizeRoleFallbacks(source []RoleFallback) []RoleFallback {
	if len(source) == 0 {
		return nil
	}
	normalized := make([]RoleFallback, len(source))
	for index, fallback := range source {
		normalized[index] = RoleFallback{
			Provider:        strings.TrimSpace(fallback.Provider),
			Model:           strings.TrimSpace(fallback.Model),
			ReasoningEffort: strings.TrimSpace(fallback.ReasoningEffort),
			Speed:           fallback.Speed,
			ACPOptions:      CloneACPOptionSelections(fallback.ACPOptions),
			Command:         strings.TrimSpace(fallback.Command),
		}
	}
	return normalized
}
