package config

import "maps"

func cloneProviders(src map[string]ProviderConfig) map[string]ProviderConfig {
	if len(src) == 0 {
		return map[string]ProviderConfig{}
	}

	cloned := make(map[string]ProviderConfig, len(src))
	for name, provider := range src {
		cloned[name] = cloneProvider(provider)
	}

	return cloned
}

func cloneProvider(src ProviderConfig) ProviderConfig {
	return ProviderConfig{
		Command:         src.Command,
		DisplayName:     src.DisplayName,
		Models:          cloneProviderModelsConfig(src.Models),
		Harness:         src.Harness,
		RuntimeProvider: src.RuntimeProvider,
		Transport:       src.Transport,
		BaseURL:         src.BaseURL,
		AuthMode:        src.AuthMode,
		EnvPolicy:       src.EnvPolicy,
		HomePolicy:      src.HomePolicy,
		NoneSecurity:    src.NoneSecurity,
		AuthStatusCmd:   src.AuthStatusCmd,
		AuthLoginCmd:    src.AuthLoginCmd,
		SessionMCP:      cloneBoolRef(src.SessionMCP),
		CredentialSlots: cloneProviderCredentialSlots(src.CredentialSlots),
		MCPServers:      cloneMCPServers(src.MCPServers),
	}
}

func cloneBoolRef(src *bool) *bool {
	if src == nil {
		return nil
	}
	return new(*src)
}

func cloneProviderCredentialSlots(src []ProviderCredentialSlot) []ProviderCredentialSlot {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]ProviderCredentialSlot, len(src))
	copy(cloned, src)
	return cloned
}

func cloneMCPServers(src []MCPServer) []MCPServer {
	return cloneMCPServersWithCapacity(src, len(src))
}

func cloneMCPServersWithCapacity(src []MCPServer, capacity int) []MCPServer {
	if len(src) == 0 {
		return nil
	}

	if capacity < len(src) {
		capacity = len(src)
	}

	cloned := make([]MCPServer, len(src), capacity)
	for i, server := range src {
		cloned[i] = cloneMCPServer(server)
	}

	return cloned
}

func cloneMCPServer(src MCPServer) MCPServer {
	return MCPServer{
		Name:           src.Name,
		Transport:      src.Transport,
		Command:        src.Command,
		Args:           append([]string(nil), src.Args...),
		Env:            mergeStringMaps(nil, src.Env),
		SecretEnv:      mergeStringMaps(nil, src.SecretEnv),
		URL:            src.URL,
		Auth:           cloneMCPAuthConfig(src.Auth),
		CatalogEntry:   src.CatalogEntry,
		CatalogVersion: src.CatalogVersion,
	}
}

func cloneMCPAuthConfig(src MCPAuthConfig) MCPAuthConfig {
	src.Scopes = append([]string(nil), src.Scopes...)
	return src
}

func mergeStringMaps(base map[string]string, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(overlay))
	maps.Copy(merged, base)
	maps.Copy(merged, overlay)

	return merged
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	return append([]string(nil), values...)
}
