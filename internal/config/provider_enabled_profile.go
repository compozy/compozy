package config

import (
	"errors"
	"fmt"
	"strings"
)

// WithProfileProviders returns base with the selected profile's [providers.<id>] tables applied on
// top (BR-2: the profile layer wins over global). A provider the profile defines on its own joins
// the inventory with its full definition and enabled state. Only that profile's own file is read,
// so a pending global edit never leaks in through the profile (B-013). A missing profile name or
// file leaves base unchanged.
func WithProfileProviders(base *Config, homePaths HomePaths, profileName string) (Config, error) {
	if base == nil {
		return Config{}, errors.New("config: base config is required")
	}
	resolved := *base
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return resolved, nil
	}
	if err := ValidateResourceProfileName(profileName); err != nil {
		return Config{}, fmt.Errorf("config: profile providers: %w", err)
	}
	overlay, err := loadPersistedConfigOverlay(profileConfigFile(homePaths, profileName), loadProfileConfigOverlayBytes)
	if err != nil {
		return Config{}, fmt.Errorf("config: load profile %q config: %w", profileName, err)
	}
	resolved.Providers = CloneProviderConfigs(base.Providers)
	applyProviderOverlays(&resolved, overlay.Providers)
	builtins := BuiltinProviders()
	for name := range overlay.Providers {
		providerName := CanonicalProviderName(name)
		if _, known := base.Providers[providerName]; known {
			continue
		}
		if _, builtin := builtins[providerName]; builtin {
			continue
		}
		if _, err := resolved.ResolveProvider(providerName); err != nil {
			// An incomplete profile-only entry (for example enabled without a command) would make
			// the inventory unresolvable; it stays out until the profile defines it fully.
			delete(resolved.Providers, providerName)
		}
	}
	return resolved, nil
}
