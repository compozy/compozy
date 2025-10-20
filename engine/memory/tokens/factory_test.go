package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

func TestNewCounterFactory(t *testing.T) {
	t.Run("Should create factory with fallback function", func(t *testing.T) {
		fallbackFunc := func() (memcore.TokenCounter, error) {
			return &mockTokenCounter{}, nil
		}
		factory := NewCounterFactory(fallbackFunc)
		assert.NotNil(t, factory)
		assert.NotNil(t, factory.GetRegistry())
		// Should have default providers registered
		providers := factory.GetRegistry().List()
		assert.True(t, len(providers) > 0)
	})
}

func TestCounterFactory_CreateCounter(t *testing.T) {
	fallbackFunc := func() (memcore.TokenCounter, error) {
		return &mockTokenCounter{
			encodingFunc: func() string {
				return "test-encoding"
			},
		}, nil
	}
	t.Run("Should create fallback counter when no config", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		counter, err := factory.CreateCounter(t.Context(), nil)
		require.NoError(t, err)
		assert.NotNil(t, counter)
		// Should be the fallback counter
		assert.Equal(t, "test-encoding", counter.GetEncoding())
	})
	t.Run("Should create unified counter with valid config", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		config := &memcore.TokenProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
			Settings: map[string]string{
				"timeout": "30s",
			},
		}
		counter, err := factory.CreateCounter(t.Context(), config)
		require.NoError(t, err)
		assert.NotNil(t, counter)
		// Should be unified counter
		assert.Equal(t, "OpenAI-gpt-4", counter.GetEncoding())
	})
	t.Run("Should fail with empty provider", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		config := &memcore.TokenProviderConfig{
			Provider: "", // Empty provider
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := factory.CreateCounter(t.Context(), config)
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "provider cannot be empty")
	})
	t.Run("Should fail with empty model", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		config := &memcore.TokenProviderConfig{
			Provider: "OpenAI",
			Model:    "", // Empty model
			APIKey:   "test-key",
		}
		counter, err := factory.CreateCounter(t.Context(), config)
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "model cannot be empty")
	})
	t.Run("Should handle fallback factory error", func(t *testing.T) {
		failingFallbackFunc := func() (memcore.TokenCounter, error) {
			return nil, assert.AnError
		}
		factory := NewCounterFactory(failingFallbackFunc)
		counter, err := factory.CreateCounter(t.Context(), nil)
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "failed to create fallback counter")
	})
}

func TestCounterFactory_CreateCounterFromRegistryKey(t *testing.T) {
	fallbackFunc := func() (memcore.TokenCounter, error) {
		return &mockTokenCounter{
			encodingFunc: func() string {
				return "test-encoding"
			},
		}, nil
	}
	t.Run("Should create counter from registry key", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		// Use a default registry key
		counter, err := factory.CreateCounterFromRegistryKey(t.Context(), "openai-gpt4", "test-api-key")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		// Should be unified counter with OpenAI configuration
		assert.Equal(t, "OpenAI-gpt-4", counter.GetEncoding())
	})
	t.Run("Should fail with non-existent registry key", func(t *testing.T) {
		factory := NewCounterFactory(fallbackFunc)
		counter, err := factory.CreateCounterFromRegistryKey(t.Context(), "non-existent", "test-api-key")
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "failed to get provider config from registry")
	})
	t.Run("Should handle fallback factory error", func(t *testing.T) {
		failingFallbackFunc := func() (memcore.TokenCounter, error) {
			return nil, assert.AnError
		}
		factory := NewCounterFactory(failingFallbackFunc)
		counter, err := factory.CreateCounterFromRegistryKey(t.Context(), "openai-gpt4", "test-api-key")
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "failed to create fallback counter")
	})
}

func TestCounterFactory_GetRegistry(t *testing.T) {
	t.Run("Should return the provider registry", func(t *testing.T) {
		fallbackFunc := func() (memcore.TokenCounter, error) {
			return &mockTokenCounter{}, nil
		}
		factory := NewCounterFactory(fallbackFunc)
		registry := factory.GetRegistry()
		assert.NotNil(t, registry)
		// Should have default providers
		providers := registry.List()
		assert.True(t, len(providers) > 0)
		assert.Contains(t, providers, "openai-gpt4")
	})
}

func TestCounterFactory_Integration(t *testing.T) {
	t.Run("Should work with real tiktoken fallback", func(t *testing.T) {
		factory := NewCounterFactory(DefaultTokenCounter)
		// Test fallback counter creation
		counter, err := factory.CreateCounter(t.Context(), nil)
		require.NoError(t, err)
		assert.NotNil(t, counter)
		// Test unified counter creation with provider config
		config := &memcore.TokenProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		unifiedCounter, err := factory.CreateCounter(t.Context(), config)
		require.NoError(t, err)
		assert.NotNil(t, unifiedCounter)
		// Test registry-based creation
		registryCounter, err := factory.CreateCounterFromRegistryKey(
			t.Context(),
			"anthropic-claude",
			"test-key",
		)
		require.NoError(t, err)
		assert.NotNil(t, registryCounter)
		assert.Equal(t, "Anthropic-claude-3-haiku", registryCounter.GetEncoding())
	})
}
