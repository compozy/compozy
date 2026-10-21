package config

import "time"

type providerUsageOverlay struct {
	Enabled         *bool                                   `toml:"enabled"`
	RefreshInterval *time.Duration                          `toml:"refresh_interval"`
	Providers       map[string]providerUsageProviderOverlay `toml:"providers"`
}

type providerUsageProviderOverlay struct {
	Enabled        *bool `toml:"enabled"`
	KeychainAccess *bool `toml:"keychain_access"`
}

func (o providerUsageOverlay) Apply(dst *ProviderUsageConfig) error {
	applyOptional(o.Enabled, &dst.Enabled)
	applyOptional(o.RefreshInterval, &dst.RefreshInterval)
	if len(o.Providers) > 0 && dst.Providers == nil {
		dst.Providers = make(map[string]ProviderUsageProviderConfig, len(o.Providers))
	}
	for id, overlay := range o.Providers {
		if id != providerClaudeKey && overlay.KeychainAccess != nil {
			return ValidationError{
				Path:    "provider_usage.providers." + id + ".keychain_access",
				Message: "is only valid for claude",
			}
		}
		provider, ok := dst.Providers[id]
		if !ok {
			provider.Enabled = true
		}
		applyOptional(overlay.Enabled, &provider.Enabled)
		applyOptional(overlay.KeychainAccess, &provider.KeychainAccess)
		dst.Providers[id] = provider
	}
	return nil
}
