package config

import (
	"maps"
	"slices"
)

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
		ReviewerMode:        src.ReviewerMode,
		SessionEnv:          MergeSessionEnv(nil, src.SessionEnv),
		AutoCompactWindow:   cloneAutoCompactWindow(src.AutoCompactWindow),
		Command:             src.Command,
		CommandIsBuiltin:    src.CommandIsBuiltin,
		DisplayName:         src.DisplayName,
		Models:              cloneProviderModelsConfig(src.Models),
		Harness:             src.Harness,
		RuntimeProvider:     src.RuntimeProvider,
		Transport:           src.Transport,
		BaseURL:             src.BaseURL,
		AuthMode:            src.AuthMode,
		EnvPolicy:           src.EnvPolicy,
		HomePolicy:          src.HomePolicy,
		NoneSecurity:        src.NoneSecurity,
		AuthStatusCmd:       src.AuthStatusCmd,
		AuthLoginCmd:        src.AuthLoginCmd,
		AuthACPMethod:       src.AuthACPMethod,
		SessionMCP:          cloneBoolRef(src.SessionMCP),
		Enabled:             cloneBoolRef(src.Enabled),
		SessionMCPTransport: src.SessionMCPTransport,
		CredentialSlots:     cloneProviderCredentialSlots(src.CredentialSlots),
		MCPServers:          cloneMCPServers(src.MCPServers),
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
	return slices.Clone(src)
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
		InputValues:    maps.Clone(src.InputValues),
		Name:           src.Name,
		Reference:      src.Reference,
		Owner:          src.Owner,
		RuntimeName:    src.RuntimeName,
		Transport:      src.Transport,
		Command:        src.Command,
		CWD:            src.CWD,
		Args:           append([]string(nil), src.Args...),
		Env:            mergeStringMaps(nil, src.Env),
		SecretEnv:      mergeStringMaps(nil, src.SecretEnv),
		URL:            src.URL,
		Headers:        mergeStringMaps(nil, src.Headers),
		SecretHeaders:  mergeStringMaps(nil, src.SecretHeaders),
		Auth:           cloneMCPAuthConfig(src.Auth),
		CatalogEntry:   src.CatalogEntry,
		CatalogVersion: src.CatalogVersion,
		ReadOnlyTools:  append([]string(nil), src.ReadOnlyTools...),
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
