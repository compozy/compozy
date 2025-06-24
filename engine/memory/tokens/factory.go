package tokens

import (
	"fmt"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// CounterFactory creates token counters based on configuration
type CounterFactory struct {
	registry        *ProviderRegistry
	fallbackFactory func() (memcore.TokenCounter, error)
	keyResolver     *APIKeyResolver
	log             logger.Logger
}

// NewCounterFactory creates a new token counter factory
func NewCounterFactory(fallbackFactory func() (memcore.TokenCounter, error), log logger.Logger) *CounterFactory {
	registry := NewProviderRegistry()
	registry.RegisterDefaults()
	return &CounterFactory{
		registry:        registry,
		fallbackFactory: fallbackFactory,
		keyResolver:     NewAPIKeyResolver(log),
		log:             log,
	}
}

// CreateCounter creates a token counter based on the provided configuration
func (f *CounterFactory) CreateCounter(
	config *memcore.TokenProviderConfig,
) (memcore.TokenCounter, error) {
	// Create fallback counter
	fallback, err := f.fallbackFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback counter: %w", err)
	}
	if config == nil {
		// No provider config, use fallback only
		return fallback, nil
	}
	// Validate provider configuration
	if config.Provider == "" {
		return nil, fmt.Errorf("provider cannot be empty")
	}
	if config.Model == "" {
		return nil, fmt.Errorf("model cannot be empty")
	}
	// Create provider config with resolved API key
	providerConfig := f.keyResolver.ResolveProviderConfig(config)
	// Create unified counter
	counter, err := NewUnifiedTokenCounter(providerConfig, fallback, f.log)
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
	registryKey string,
	apiKeyOrEnvVar string,
) (memcore.TokenCounter, error) {
	// Create fallback counter
	fallback, err := f.fallbackFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback counter: %w", err)
	}
	// Get provider config from registry
	config, err := f.registry.Clone(registryKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config from registry: %w", err)
	}
	// Create a temporary token provider config to resolve API key
	tempConfig := &memcore.TokenProviderConfig{
		Provider: config.Provider,
		Model:    config.Model,
		APIKey:   apiKeyOrEnvVar,
		Endpoint: config.Endpoint,
		Settings: config.Settings,
	}
	// Resolve the API key (handles env vars)
	resolvedConfig := f.keyResolver.ResolveProviderConfig(tempConfig)
	// Create unified counter
	counter, err := NewUnifiedTokenCounter(resolvedConfig, fallback, f.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified counter: %w", err)
	}
	return counter, nil
}
