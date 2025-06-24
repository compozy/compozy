package tokens

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed provider_defaults.yaml
var defaultProvidersYAML []byte

// ProviderDefaults represents the structure of provider defaults configuration
type ProviderDefaults struct {
	Providers []ProviderDefault `yaml:"providers"`
}

// ProviderDefault represents a single provider default configuration
type ProviderDefault struct {
	Name     string            `yaml:"name"`
	Provider string            `yaml:"provider"`
	Model    string            `yaml:"model"`
	Settings map[string]string `yaml:"settings,omitempty"`
}

// LoadProviderDefaults loads the embedded provider defaults from YAML
func LoadProviderDefaults() (*ProviderDefaults, error) {
	var defaults ProviderDefaults
	if err := yaml.Unmarshal(defaultProvidersYAML, &defaults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider defaults: %w", err)
	}
	return &defaults, nil
}

// RegisterDefaultsFromYAML loads provider defaults from embedded YAML and registers them
func (r *ProviderRegistry) RegisterDefaultsFromYAML() error {
	defaults, err := LoadProviderDefaults()
	if err != nil {
		return fmt.Errorf("failed to load provider defaults: %w", err)
	}
	// Register each provider configuration
	for _, def := range defaults.Providers {
		config := &ProviderConfig{
			Provider: def.Provider,
			Model:    def.Model,
			Settings: def.Settings,
		}
		if err := r.Register(def.Name, config); err != nil {
			// Log warning but continue with other registrations
			// This allows partial success if some configurations have issues
			continue
		}
	}
	return nil
}
