package config

import (
	"maps"
	"slices"
	"time"
)

const ProviderUsageConfigKey = "provider_usage"
const DefaultProviderUsageRefreshInterval = 5 * time.Minute

func DefaultProviderUsageConfig() ProviderUsageConfig {
	return ProviderUsageConfig{Enabled: true, RefreshInterval: DefaultProviderUsageRefreshInterval}
}

func (c ProviderUsageConfig) ProviderEnabled(id string) bool {
	provider, ok := c.Providers[id]
	return c.Enabled && (!ok || provider.Enabled)
}

func (c ProviderUsageConfig) ClaudeKeychainAccess() bool {
	return c.Providers[providerClaudeKey].KeychainAccess
}

func (c ProviderUsageConfig) Clone() ProviderUsageConfig {
	c.Providers = maps.Clone(c.Providers)
	return c
}

func (c ProviderUsageConfig) Validate(providers map[string]ProviderConfig) error {
	if c.RefreshInterval < time.Minute || c.RefreshInterval > time.Hour {
		return ValidationError{Path: "provider_usage.refresh_interval", Message: "must be between 1m and 1h"}
	}
	for _, id := range slices.Sorted(maps.Keys(c.Providers)) {
		path := "provider_usage.providers." + id
		_, builtin := builtinProviders[id]
		_, custom := providers[id]
		if CanonicalProviderName(id) != id || (!builtin && !custom) {
			return ValidationError{Path: path, Message: "unknown canonical provider ID"}
		}
		if id != providerClaudeKey && c.Providers[id].KeychainAccess {
			return ValidationError{Path: path + ".keychain_access", Message: "is only valid for claude"}
		}
	}
	return nil
}

func providerUsageToolPath(path []string) bool {
	return len(path) == 4 && path[0] == ProviderUsageConfigKey && path[1] == "providers" &&
		(path[3] == "enabled" || (path[2] == "claude" && path[3] == "keychain_access"))
}
