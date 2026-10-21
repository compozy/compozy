package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	configProviderUsageProvidersKey = "providers"
	configProviderUsageEnabledKey   = "enabled"
	configProviderUsageKeychainKey  = "keychain_access"
)

// applyProviderUsageConfigValue sets one provider_usage key on a copy of the
// loaded config, so the daemon receives the whole section with one change.
func applyProviderUsageConfigValue(cfg *compozyconfig.ProviderUsageConfig, path []string, value any) error {
	key := strings.Join(path, ".")
	if cfg == nil || len(path) < 2 || path[0] != compozyconfig.ProviderUsageConfigKey {
		return fmt.Errorf("cli: unsupported provider usage config path %q", key)
	}
	switch {
	case len(path) == 2 && path[1] == configProviderUsageEnabledKey:
		return assignConfigValue(path, value, &cfg.Enabled, "boolean")
	case len(path) == 2 && path[1] == "refresh_interval":
		raw, ok := value.(string)
		if !ok {
			return fmt.Errorf("cli: config set %q expects a duration, got %T", key, value)
		}
		interval, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("cli: config set %q: %w", key, err)
		}
		cfg.RefreshInterval = interval
		return nil
	case len(path) == 4 && path[1] == configProviderUsageProvidersKey:
		*cfg = cfg.Clone()
		if cfg.Providers == nil {
			cfg.Providers = map[string]compozyconfig.ProviderUsageProviderConfig{}
		}
		provider, known := cfg.Providers[path[2]]
		if !known {
			provider.Enabled = true
		}
		var err error
		switch path[3] {
		case configProviderUsageEnabledKey:
			err = assignConfigValue(path, value, &provider.Enabled, "boolean")
		case configProviderUsageKeychainKey:
			err = assignConfigValue(path, value, &provider.KeychainAccess, "boolean")
		default:
			err = fmt.Errorf("cli: unsupported provider usage config path %q", key)
		}
		if err != nil {
			return err
		}
		cfg.Providers[path[2]] = provider
		return nil
	default:
		return fmt.Errorf("cli: unsupported provider usage config path %q", key)
	}
}

// settingsProviderUsagePayloadFromConfig mirrors the settings section read DTO:
// keychain_access is sent only for claude, the one provider it is valid for.
func settingsProviderUsagePayloadFromConfig(
	cfg compozyconfig.ProviderUsageConfig,
) contract.SettingsProviderUsagePayload {
	providers := make(map[string]contract.SettingsProviderUsageProviderPayload, len(cfg.Providers))
	for id, provider := range cfg.Providers {
		entry := contract.SettingsProviderUsageProviderPayload{Enabled: new(provider.Enabled)}
		if id == "claude" {
			entry.KeychainAccess = new(provider.KeychainAccess)
		}
		providers[id] = entry
	}
	return contract.SettingsProviderUsagePayload{
		Enabled:         cfg.Enabled,
		RefreshInterval: cfg.RefreshInterval.String(),
		Providers:       providers,
	}
}
