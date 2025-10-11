package llmadapter

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// DefaultFactory creates LLM clients using a provider registry.
type DefaultFactory struct {
	registry *Registry
}

// NewDefaultFactory creates a new DefaultFactory with builtin providers
// registered against a fresh registry instance.
func NewDefaultFactory(ctx context.Context) (Factory, error) {
	registry := NewProviderRegistry()
	if err := RegisterProviders(ctx, registry, BuiltinProviders()...); err != nil {
		return nil, err
	}
	return &DefaultFactory{registry: registry}, nil
}

// NewDefaultFactoryWithRegistry creates a factory bound to the provided registry.
func NewDefaultFactoryWithRegistry(registry *Registry) Factory {
	if registry == nil {
		registry = NewProviderRegistry()
	}
	return &DefaultFactory{registry: registry}
}

// CreateClient creates a new LLMClient for the given provider.
func (f *DefaultFactory) CreateClient(ctx context.Context, config *core.ProviderConfig) (LLMClient, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	return f.registry.NewClient(ctx, config)
}

// BuildRoute constructs a provider route for the supplied configs.
func (f *DefaultFactory) BuildRoute(
	config *core.ProviderConfig,
	fallbacks ...*core.ProviderConfig,
) (*ProviderRoute, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	return f.registry.BuildRoute(config, fallbacks...)
}

// Capabilities returns capability metadata for the specified provider.
func (f *DefaultFactory) Capabilities(name core.ProviderName) (ProviderCapabilities, error) {
	return f.registry.Capabilities(name)
}
