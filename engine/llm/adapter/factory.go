package llmadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	factorymetrics "github.com/compozy/compozy/engine/llm/factory/metrics"
)

// DefaultFactory creates LLM clients using a provider registry.
type DefaultFactory struct {
	registry *Registry
}

// NewDefaultFactory creates a new DefaultFactory with builtin providers
// registered against a fresh registry instance.
func NewDefaultFactory(ctx context.Context) (Factory, error) {
	start := time.Now()
	registry := NewProviderRegistry()
	if err := RegisterProviders(ctx, registry, BuiltinProviders()...); err != nil {
		return nil, err
	}
	factory := &DefaultFactory{registry: registry}
	factorymetrics.RecordCreate(ctx, factorymetrics.TypeProvider, "default", time.Since(start))
	return factory, nil
}

// NewDefaultFactoryWithRegistry creates a factory bound to the provided registry.
// If registry is nil, a new empty registry is created WITHOUT builtin providers.
// Callers must explicitly register providers via RegisterProviders() after creation.
// For a factory with builtin providers pre-registered, use NewDefaultFactory() instead.
func NewDefaultFactoryWithRegistry(ctx context.Context, registry *Registry) Factory {
	if ctx == nil {
		panic("context must not be nil")
	}
	start := time.Now()
	if registry == nil {
		registry = NewProviderRegistry()
	}
	factory := &DefaultFactory{registry: registry}
	factorymetrics.RecordCreate(ctx, factorymetrics.TypeProvider, "custom_registry", time.Since(start))
	return factory
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
