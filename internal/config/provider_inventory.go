package config

import (
	"maps"
	"slices"
)

// ProviderInventoryNames lists built-in and configured provider names, sorted and canonical.
func ProviderInventoryNames(cfg *Config) []string {
	seen := make(map[string]struct{})
	for name := range BuiltinProviders() {
		seen[name] = struct{}{}
	}
	if cfg != nil {
		for name := range cfg.Providers {
			if canonical := CanonicalProviderName(name); canonical != "" {
				seen[canonical] = struct{}{}
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}
