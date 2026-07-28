package config

import (
	"fmt"

	"strings"
)

func mergeProvider(base ProviderConfig, override ProviderConfig) ProviderConfig {
	merged := cloneProvider(base)
	if strings.TrimSpace(override.Command) != "" {
		merged.Command = override.Command
	}
	if strings.TrimSpace(override.DisplayName) != "" {
		merged.DisplayName = override.DisplayName
	}
	if !providerModelsConfigIsZero(override.Models) {
		merged.Models = mergeProviderModels(merged.Models, override.Models)
	}
	if override.Harness != "" {
		merged.Harness = override.Harness
	}
	if strings.TrimSpace(override.RuntimeProvider) != "" {
		merged.RuntimeProvider = override.RuntimeProvider
	}
	if strings.TrimSpace(override.Transport) != "" {
		merged.Transport = override.Transport
	}
	if strings.TrimSpace(override.BaseURL) != "" {
		merged.BaseURL = override.BaseURL
	}
	applyProviderAuthModeOverride(&merged, override)
	if override.EnvPolicy != "" {
		merged.EnvPolicy = override.EnvPolicy
	}
	if override.HomePolicy != "" {
		merged.HomePolicy = override.HomePolicy
	}
	if override.NoneSecurity != "" {
		merged.NoneSecurity = override.NoneSecurity
	}
	if strings.TrimSpace(override.AuthStatusCmd) != "" {
		merged.AuthStatusCmd = override.AuthStatusCmd
	}
	if strings.TrimSpace(override.AuthLoginCmd) != "" {
		merged.AuthLoginCmd = override.AuthLoginCmd
	}
	if override.SessionMCP != nil {
		merged.SessionMCP = new(*override.SessionMCP)
	}
	if len(override.CredentialSlots) > 0 {
		merged.CredentialSlots = cloneProviderCredentialSlots(override.CredentialSlots)
	}
	merged.MCPServers = MergeMCPServers(merged.MCPServers, override.MCPServers)

	return merged
}

func mergeProviderModels(base ProviderModelsConfig, override ProviderModelsConfig) ProviderModelsConfig {
	merged := cloneProviderModelsConfig(base)
	if strings.TrimSpace(override.Default) != "" {
		merged.Default = override.Default
	}
	if override.Curated != nil {
		merged.Curated = cloneProviderModelConfigs(override.Curated)
	}
	if !providerModelsDiscoveryConfigIsZero(override.Discovery) {
		merged.Discovery = mergeProviderModelsDiscovery(merged.Discovery, override.Discovery)
	}
	if !providerReasoningConfigIsZero(override.Reasoning) {
		merged.Reasoning = mergeProviderReasoning(merged.Reasoning, override.Reasoning)
	}
	return merged
}

func mergeProviderModelsDiscovery(
	base ProviderModelsDiscoveryConfig,
	override ProviderModelsDiscoveryConfig,
) ProviderModelsDiscoveryConfig {
	merged := cloneProviderModelsDiscoveryConfig(base)
	if override.Enabled != nil {
		merged.Enabled = new(*override.Enabled)
	}
	if strings.TrimSpace(override.Command) != "" {
		merged.Command = override.Command
	}
	if strings.TrimSpace(override.Endpoint) != "" {
		merged.Endpoint = override.Endpoint
	}
	if strings.TrimSpace(override.Timeout) != "" {
		merged.Timeout = override.Timeout
	}
	return merged
}

func providerModelsConfigIsZero(value ProviderModelsConfig) bool {
	return strings.TrimSpace(value.Default) == "" &&
		value.Curated == nil &&
		providerModelsDiscoveryConfigIsZero(value.Discovery) &&
		providerReasoningConfigIsZero(value.Reasoning)
}

func providerModelsDiscoveryConfigIsZero(value ProviderModelsDiscoveryConfig) bool {
	return value.Enabled == nil &&
		strings.TrimSpace(value.Command) == "" &&
		strings.TrimSpace(value.Endpoint) == "" &&
		strings.TrimSpace(value.Timeout) == ""
}

func newUnknownProviderError(providerName string) error {
	return fmt.Errorf("%w: unknown provider %q", ErrProviderUnavailable, providerName)
}

// MergeMCPServers merges provider-level and agent-level MCP servers by name.
func MergeMCPServers(base []MCPServer, overlay []MCPServer) []MCPServer {
	return mergeMCPServerLayers(base, overlay)
}

// OverrideMCPServers overlays MCP servers by name, replacing the full server object
// on collision instead of field-merging it.
func OverrideMCPServers(base []MCPServer, overlay []MCPServer) []MCPServer {
	merged := cloneMCPServersWithCapacity(base, len(base)+len(overlay))
	index := indexMCPServersByName(merged)

	for _, server := range overlay {
		name := normalizeMCPServerName(server.Name)
		if idx, ok := index[name]; ok && name != "" {
			merged[idx] = cloneMCPServer(server)
			continue
		}

		merged = append(merged, cloneMCPServer(server))
		if name != "" {
			index[name] = len(merged) - 1
		}
	}

	return merged
}

func mergeMCPServerLayers(base []MCPServer, overlays ...[]MCPServer) []MCPServer {
	totalCapacity := len(base)
	for _, overlay := range overlays {
		totalCapacity += len(overlay)
	}

	merged := cloneMCPServersWithCapacity(base, totalCapacity)
	index := indexMCPServersByName(merged)

	for _, overlay := range overlays {
		for _, server := range overlay {
			name := normalizeMCPServerName(server.Name)
			if idx, ok := index[name]; ok && name != "" {
				mergeMCPServerInto(&merged[idx], server)
				continue
			}

			merged = append(merged, cloneMCPServer(server))
			if name != "" {
				index[name] = len(merged) - 1
			}
		}
	}

	return merged
}

func normalizeMCPServerName(name string) string {
	return strings.TrimSpace(name)
}

func indexMCPServersByName(servers []MCPServer) map[string]int {
	index := make(map[string]int, len(servers))
	for i, server := range servers {
		name := normalizeMCPServerName(server.Name)
		if name == "" {
			continue
		}
		index[name] = i
	}
	return index
}
