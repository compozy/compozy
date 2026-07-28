package extensionpkg

import "strings"

func normalizeMCPServers(src map[string]MCPServerConfig) map[string]MCPServerConfig {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]MCPServerConfig, len(src))
	for _, name := range sortedMapKeys(src) {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}

		server := src[name]
		dst[trimmedName] = MCPServerConfig{
			Command:   strings.TrimSpace(server.Command),
			Args:      normalizeStrings(server.Args),
			Env:       normalizeStringMap(server.Env),
			SecretEnv: normalizeStringMap(server.SecretEnv),
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func normalizeTools(src map[string]ToolConfig) map[string]ToolConfig {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]ToolConfig, len(src))
	for _, name := range sortedMapKeys(src) {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}

		tool := src[name]
		dst[trimmedName] = ToolConfig{
			ID:           strings.TrimSpace(tool.ID),
			DisplayTitle: strings.TrimSpace(tool.DisplayTitle),
			FriendlyVerb: strings.TrimSpace(tool.FriendlyVerb),
			Preview:      strings.TrimSpace(tool.Preview),
			Description:  strings.TrimSpace(tool.Description),
			Handler:      strings.TrimSpace(tool.Handler),
			Backend: ToolBackendConfig{
				Kind:    strings.TrimSpace(tool.Backend.Kind),
				Handler: strings.TrimSpace(tool.Backend.Handler),
				Server:  strings.TrimSpace(tool.Backend.Server),
				Tool:    strings.TrimSpace(tool.Backend.Tool),
			},
			InputSchema:          cloneManifestRawMessage(tool.InputSchema),
			OutputSchema:         cloneManifestRawMessage(tool.OutputSchema),
			Risk:                 strings.TrimSpace(tool.Risk),
			ReadOnly:             tool.ReadOnly,
			Destructive:          tool.Destructive,
			OpenWorld:            tool.OpenWorld,
			RequiresInteraction:  tool.RequiresInteraction,
			ConcurrencySafe:      tool.ConcurrencySafe,
			MaxResultBytes:       tool.MaxResultBytes,
			Toolsets:             normalizeStrings(tool.Toolsets),
			Tags:                 normalizeStrings(tool.Tags),
			SearchHints:          normalizeStrings(tool.SearchHints),
			RequiresEnv:          normalizeStrings(tool.RequiresEnv),
			RequiredCapabilities: normalizeStrings(tool.RequiredCapabilities),
			Visibility:           strings.TrimSpace(tool.Visibility),
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}
