package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

// WithProfileProviders returns base with the selected profile's providers layered on top (BR-2:
// the profile layer wins over global). A provider base already knows (configured or built in)
// takes only the profile's providers.<id>.enabled, so its identity, and any sign-in verdict
// keyed by it, stays the global one. A provider the profile defines on its own joins with its full
// [providers.<id>] table; an incomplete one that would not resolve (for example enabled without a
// command) stays out. Only that profile's own file is read, without migrating or rewriting it, so
// a pending global edit never leaks in through the profile (B-013). A missing profile name or file
// leaves base unchanged.
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
	content, err := os.ReadFile(profileConfigFile(homePaths, profileName))
	if errors.Is(err, os.ErrNotExist) {
		return resolved, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: read profile %q config: %w", profileName, err)
	}
	var decoded struct {
		Providers map[string]providerOverlay `toml:"providers"`
	}
	if _, err := burnttoml.Decode(string(content), &decoded); err != nil {
		return Config{}, fmt.Errorf("config: decode profile %q config: %w", profileName, err)
	}
	resolved.Providers = CloneProviderConfigs(base.Providers)
	if resolved.Providers == nil {
		resolved.Providers = make(map[string]ProviderConfig)
	}
	builtins := BuiltinProviders()
	for name, overlay := range decoded.Providers {
		providerName := CanonicalProviderName(name)
		if providerName == "" {
			continue
		}
		provider, known := resolved.Providers[providerName]
		if _, builtin := builtins[providerName]; known || builtin {
			if overlay.Enabled != nil {
				provider.Enabled = new(*overlay.Enabled)
				resolved.Providers[providerName] = provider
			}
			continue
		}
		overlay.Apply(&provider)
		resolved.Providers[providerName] = provider
		if _, err := resolved.ResolveProvider(providerName); err != nil {
			delete(resolved.Providers, providerName)
		}
	}
	return resolved, nil
}
