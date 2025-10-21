package tokens

import (
	"context"
	"fmt"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

// CounterFactory creates token counters based on configuration
type CounterFactory struct {
	registry        *ProviderRegistry
	fallbackFactory func() (memcore.TokenCounter, error)
	keyResolver     *APIKeyResolver
}

// NewCounterFactory creates a new token counter factory
func NewCounterFactory(fallbackFactory func() (memcore.TokenCounter, error)) *CounterFactory {
	registry := NewProviderRegistry()
	registry.RegisterDefaults()
	return &CounterFactory{
		registry:        registry,
		fallbackFactory: fallbackFactory,
		keyResolver:     NewAPIKeyResolver(),
	}
}

// CreateCounter creates a token counter based on the provided configuration
func (f *CounterFactory) CreateCounter(
	ctx context.Context,
	config *memcore.TokenProviderConfig,
) (memcore.TokenCounter, error) {
	fallback, err := f.fallbackFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback counter: %w", err)
	}
	if config == nil {
		return fallback, nil
	}
	if config.Provider == "" {
		return nil, fmt.Errorf("provider cannot be empty")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	providerConfig := f.keyResolver.ResolveProviderConfig(ctx, config)
	counter, err := NewUnifiedTokenCounter(providerConfig, fallback)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified counter: %w", err)
	}
	return counter, nil
}

// GetRegistry returns the provider registry for inspection or additional configuration
func (f *CounterFactory) GetRegistry() *ProviderRegistry {
	return f.registry
}

// CreateCounterFromRegistryKey creates a counter using a predefined provider configuration
func (f *CounterFactory) CreateCounterFromRegistryKey(
	ctx context.Context,
	registryKey string,
	apiKeyOrEnvVar string,
) (memcore.TokenCounter, error) {
	fallback, err := f.fallbackFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback counter: %w", err)
	}
	config, err := f.registry.Clone(registryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config from registry: %w", err)
	}
	tempConfig := &memcore.TokenProviderConfig{
		Provider: config.Provider,
		Model:    config.Model,
		APIKey:   apiKeyOrEnvVar,
		Endpoint: config.Endpoint,
		Settings: config.Settings,
	}
	resolvedConfig := f.keyResolver.ResolveProviderConfig(ctx, tempConfig)
	counter, err := NewUnifiedTokenCounter(resolvedConfig, fallback)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified counter: %w", err)
	}
	return counter, nil
}
