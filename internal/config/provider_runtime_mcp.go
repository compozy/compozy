package config

import (
	"fmt"
	"slices"
)

func (p ProviderConfig) EffectiveRuntimeMCP() string {
	if p.RuntimeMCP == "" {
		return "load"
	}
	return p.RuntimeMCP
}

// CanDisableRuntimeMCP reports whether the provider's runtime can be launched without its own MCP servers.
// Aliases resolve to the builtin provider id first, so "qwen" and "qwen-code" agree.
func CanDisableRuntimeMCP(provider string) bool {
	return slices.Contains([]string{providerClaudeKey, "copilot", "qwen-code"}, CanonicalProviderName(provider))
}

func validateRuntimeMCP(name string, provider ProviderConfig) error {
	switch provider.EffectiveRuntimeMCP() {
	case "load":
		return nil
	case "off":
		if CanDisableRuntimeMCP(name) {
			return nil
		}
		display := provider.DisplayName
		if display == "" {
			display = name
		}
		return fmt.Errorf(
			"runtime_mcp_unsupported: %s always loads its own MCP servers; runtime_mcp can only be \"load\"",
			display,
		)
	default:
		return fmt.Errorf("providers.%s.runtime_mcp must be \"load\" or \"off\"", name)
	}
}
