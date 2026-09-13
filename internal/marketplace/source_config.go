package marketplace

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const (
	SourceKindFeed   = "feed"
	SourceKindPreset = "preset"
	SourceKindCustom = "custom"
)

var ErrSourceExists = errors.New("marketplace_source_exists")

type ResolvedSource struct {
	Name        string
	Ref         string
	Kind        string
	Description string
	Enabled     bool
}

// ResolveSources applies choices by source identity and preserves feed, preset and custom ordering.
func ResolveSources(cfg config.MarketplaceRuntimeConfig, presets []Preset) ([]ResolvedSource, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	configured := make(map[string]config.MarketplacePluginSourceConfig, len(cfg.PluginSources))
	customOrder := make([]string, 0, len(cfg.PluginSources))
	for _, source := range cfg.PluginSources {
		ref, err := pluginsource.NormalizeRef(source.Source)
		if err != nil {
			return nil, err
		}
		configured[ref] = source
		customOrder = append(customOrder, ref)
	}
	sources := []ResolvedSource{
		{Name: CompozyCatalogSource, Ref: CompozyCatalogRef, Kind: SourceKindFeed, Enabled: true},
	}
	names, refs := map[string]bool{CompozyCatalogSource: true}, make(map[string]bool, len(presets))
	for _, preset := range presets {
		if err := pluginsource.ValidateName(preset.Name); err != nil {
			return nil, err
		}
		ref, err := pluginsource.NormalizeRef(preset.Source)
		if err != nil {
			return nil, err
		}
		if names[preset.Name] || refs[ref] {
			return nil, fmt.Errorf("%w: duplicate preset %q", ErrSourceExists, preset.Name)
		}
		if preset.Default != "on" && preset.Default != "off" {
			return nil, fmt.Errorf("marketplace preset %q default must be on or off", preset.Name)
		}
		names[preset.Name], refs[ref] = true, true
		enabled := preset.Default == "on"
		if override, exists := configured[ref]; exists {
			enabled = override.EffectiveEnabled(enabled)
			delete(configured, ref)
		}
		sources = append(sources, ResolvedSource{
			Name: preset.Name, Ref: ref, Kind: SourceKindPreset, Description: preset.Description, Enabled: enabled,
		})
	}
	for _, ref := range customOrder {
		source, exists := configured[ref]
		if !exists {
			continue
		}
		if names[source.Name] {
			return nil, fmt.Errorf("%w: custom name %q collides with a preset", ErrSourceExists, source.Name)
		}
		names[source.Name] = true
		sources = append(
			sources,
			ResolvedSource{Name: source.Name, Ref: ref, Kind: SourceKindCustom, Enabled: source.EffectiveEnabled(true)},
		)
	}
	return sources, nil
}
