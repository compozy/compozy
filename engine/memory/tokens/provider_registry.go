package tokens

import (
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/core"
)

// ProviderRegistry manages provider configurations for token counting
type ProviderRegistry struct {
	providers map[string]*ProviderConfig
	mu        sync.RWMutex
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]*ProviderConfig),
	}
}

// Register adds or updates a provider configuration
func (r *ProviderRegistry) Register(name string, config *ProviderConfig) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if config == nil {
		return fmt.Errorf("provider config cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = config
	return nil
}

// Get retrieves a provider configuration by name
func (r *ProviderRegistry) Get(name string) (*ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	config, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}
	return config, nil
}

// List returns all registered provider names
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providers = append(providers, name)
	}
	return providers
}

// Remove deletes a provider configuration
func (r *ProviderRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// RegisterDefaults registers commonly used provider configurations from embedded YAML
func (r *ProviderRegistry) RegisterDefaults() {
	// NOTE: Defaults are best-effort; ignore errors so registry remains usable without YAML.
	//nolint:errcheck // Default YAML may be missing in trimmed builds; skip hard failure.
	r.RegisterDefaultsFromYAML()
}

// Clone creates a deep copy of a provider configuration
func (r *ProviderRegistry) Clone(name string) (*ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	config, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}
	cloned := &ProviderConfig{
		Provider: config.Provider,
		Model:    config.Model,
		APIKey:   config.APIKey,
		Endpoint: config.Endpoint,
		Settings: core.CloneMap(config.Settings),
	}
	return cloned, nil
}
