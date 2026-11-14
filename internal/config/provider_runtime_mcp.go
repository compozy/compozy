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

func CanDisableRuntimeMCP(provider string) bool {
	return slices.Contains([]string{"claude", "copilot", "qwen"}, provider)
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
		return fmt.Errorf("runtime_mcp_unsupported: %s always loads its own MCP servers; runtime_mcp can only be \"load\"", display)
	default:
		return fmt.Errorf("providers.%s.runtime_mcp must be \"load\" or \"off\"", name)
	}
}
