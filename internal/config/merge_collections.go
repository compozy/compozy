package config

import (
	"sort"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

func applyProviderOverlays(dst *Config, overlays map[string]providerOverlay) {
	if len(overlays) == 0 {
		return
	}
	if dst.Providers == nil {
		dst.Providers = make(map[string]ProviderConfig, len(overlays))
	}

	for name, overlay := range overlays {
		providerName := CanonicalProviderName(name)
		if providerName == "" {
			continue
		}
		provider := dst.Providers[providerName]
		overlay.Apply(&provider)
		dst.Providers[providerName] = provider
	}
}

func applySandboxOverlays(dst *Config, overlays map[string]sandboxOverlay) {
	if len(overlays) == 0 {
		return
	}
	if dst.Sandboxes == nil {
		dst.Sandboxes = make(map[string]SandboxProfile, len(overlays))
	}

	for name, overlay := range overlays {
		profile := dst.Sandboxes[name]
		overlay.Apply(&profile)
		dst.Sandboxes[name] = profile
	}
}

func applyMCPServerOverlays(base []MCPServer, overlays []mcpServerOverlay) []MCPServer {
	merged := cloneMCPServers(base)
	index := indexMCPServersByName(merged)

	for _, overlay := range overlays {
		name := ""
		if overlay.Name != nil {
			name = normalizeMCPServerName(*overlay.Name)
		}

		if idx, ok := index[name]; ok && name != "" {
			server := merged[idx]
			overlay.Apply(&server)
			merged[idx] = server
			if normalized := normalizeMCPServerName(server.Name); normalized != name {
				delete(index, name)
				if normalized != "" {
					index[normalized] = idx
				}
			}
			continue
		}

		var server MCPServer
		overlay.Apply(&server)
		merged = append(merged, server)
		if normalized := normalizeMCPServerName(server.Name); normalized != "" {
			index[normalized] = len(merged) - 1
		}
	}

	return merged
}

func joinTOMLKeys(keys []burnttoml.Key) string {
	if len(keys) == 0 {
		return ""
	}

	sorted := sortedTOMLKeys(keys)
	values := make([]string, 0, len(sorted))
	for _, key := range sorted {
		values = append(values, key.String())
	}

	return strings.Join(values, ", ")
}

func sortedTOMLKeys(keys []burnttoml.Key) []burnttoml.Key {
	sorted := append([]burnttoml.Key(nil), keys...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].String() < sorted[j].String()
	})
	return sorted
}
