package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

type MarketplacePluginSourceConfig struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
	// Nil inherits a preset's default or enables a custom source.
	Enabled *bool `toml:"enabled,omitempty"`
}

func (c MarketplacePluginSourceConfig) EffectiveEnabled(presetDefault bool) bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return presetDefault
}

func ValidateMarketplacePluginSources(sources []MarketplacePluginSourceConfig) error {
	names := make(map[string]struct{}, len(sources))
	for index, source := range sources {
		path := fmt.Sprintf("marketplace.plugin_sources[%d]", index)
		if err := pluginsource.ValidateName(source.Name); err != nil {
			return fmt.Errorf("%s.name: %w", path, err)
		}
		if _, exists := names[source.Name]; exists {
			return fmt.Errorf("%s.name: marketplace_source_exists: duplicate name %q", path, source.Name)
		}
		names[source.Name] = struct{}{}
		sourceRef := strings.TrimSpace(source.Source)
		if !strings.HasPrefix(sourceRef, "github:") && !strings.HasPrefix(sourceRef, "git+https:") &&
			!strings.HasPrefix(sourceRef, "file:") {
			return fmt.Errorf("%s.source: %w", path, pluginsource.ErrInvalidRef)
		}
		_, err := pluginsource.NormalizeRef(sourceRef)
		if err != nil {
			return fmt.Errorf("%s.source: %w", path, err)
		}
	}
	return nil
}

func cloneMarketplacePluginSources(sources []MarketplacePluginSourceConfig) []MarketplacePluginSourceConfig {
	cloned := slices.Clone(sources)
	for index := range cloned {
		cloned[index].Enabled = cloneBoolPtr(sources[index].Enabled)
	}
	return cloned
}
