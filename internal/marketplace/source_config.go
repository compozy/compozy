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

var ErrSourcePresetReadonly = errors.New("marketplace_source_preset_readonly")

type SourceExistsError struct {
	Name          string
	SuggestedName string
}

func (e *SourceExistsError) Error() string { return fmt.Sprintf("%s: %s", ErrSourceExists, e.Name) }
func (e *SourceExistsError) Unwrap() error { return ErrSourceExists }

type ResolvedSource struct {
	Revision    string
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
	configured := make([]ResolvedSource, 0, len(cfg.PluginSources))
	for _, source := range cfg.PluginSources {
		ref, err := pluginsource.NormalizeRef(source.Source)
		if err != nil {
			return nil, err
		}
		configured = append(configured, ResolvedSource{
			Name: source.Name, Ref: ref, Kind: SourceKindCustom, Enabled: source.EffectiveEnabled(true),
		})
	}
	consumed := make([]bool, len(configured))
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
		// Prefer the current preset name; otherwise retain the first ref-based choice after a rename.
		override := -1
		for index, source := range configured {
			if source.Ref == ref && (override == -1 || source.Name == preset.Name) {
				override = index
				if source.Name == preset.Name {
					break
				}
			}
		}
		if override >= 0 {
			enabled = cfg.PluginSources[override].EffectiveEnabled(enabled)
			consumed[override] = true
		}
		sources = append(sources, ResolvedSource{
			Name: preset.Name, Ref: ref, Kind: SourceKindPreset, Description: preset.Description, Enabled: enabled,
		})
	}
	for index, source := range configured {
		if consumed[index] {
			continue
		}
		if names[source.Name] {
			return nil, fmt.Errorf("%w: custom name %q collides with a preset", ErrSourceExists, source.Name)
		}
		names[source.Name] = true
		sources = append(sources, source)
	}
	return sources, nil
}
