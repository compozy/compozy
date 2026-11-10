package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

// WithProfileProviderEnabled returns base with the selected profile's providers.<id>.enabled
// overrides applied on top (BR-2: the profile layer wins over global). Only that profile's own
// file is read, so a pending global edit never leaks in through the profile (B-013). A missing
// profile name or file leaves base unchanged.
func WithProfileProviderEnabled(base *Config, homePaths HomePaths, profileName string) (Config, error) {
	if base == nil {
		return Config{}, errors.New("config: base config is required")
	}
	resolved := *base
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return resolved, nil
	}
	if err := ValidateResourceProfileName(profileName); err != nil {
		return Config{}, fmt.Errorf("config: profile provider overrides: %w", err)
	}
	content, err := os.ReadFile(profileConfigFile(homePaths, profileName))
	if errors.Is(err, os.ErrNotExist) {
		return resolved, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: read profile %q config: %w", profileName, err)
	}
	var decoded struct {
		Providers map[string]struct {
			Enabled *bool `toml:"enabled"`
		} `toml:"providers"`
	}
	if _, err := burnttoml.Decode(string(content), &decoded); err != nil {
		return Config{}, fmt.Errorf("config: decode profile %q config: %w", profileName, err)
	}
	providers := CloneProviderConfigs(base.Providers)
	if providers == nil {
		providers = make(map[string]ProviderConfig)
	}
	builtins := BuiltinProviders()
	for name, overlay := range decoded.Providers {
		if overlay.Enabled == nil {
			continue
		}
		providerName := CanonicalProviderName(name)
		provider, known := providers[providerName]
		if _, builtin := builtins[providerName]; !known && !builtin {
			// A provider defined only in the profile is outside base's inventory; inserting an
			// enabled-only entry would make it unresolvable (no command).
			continue
		}
		provider.Enabled = new(*overlay.Enabled)
		providers[providerName] = provider
	}
	resolved.Providers = providers
	return resolved, nil
}
