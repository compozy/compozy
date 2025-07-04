package tokens

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderRegistry(t *testing.T) {
	t.Run("Should create empty registry", func(t *testing.T) {
		registry := NewProviderRegistry()
		assert.NotNil(t, registry)
		assert.Empty(t, registry.List())
	})
}

func TestProviderRegistry_Register(t *testing.T) {
	t.Run("Should register provider successfully", func(t *testing.T) {
		registry := NewProviderRegistry()
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		err := registry.Register("openai-gpt4", config)
		require.NoError(t, err)
		providers := registry.List()
		assert.Contains(t, providers, "openai-gpt4")
	})
	t.Run("Should fail with empty name", func(t *testing.T) {
		registry := NewProviderRegistry()
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
		}
		err := registry.Register("", config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider name cannot be empty")
	})
	t.Run("Should fail with nil config", func(t *testing.T) {
		registry := NewProviderRegistry()
		err := registry.Register("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider config cannot be nil")
	})
	t.Run("Should overwrite existing provider", func(t *testing.T) {
		registry := NewProviderRegistry()
		config1 := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-3.5",
		}
		config2 := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
		}
		err := registry.Register("openai", config1)
		require.NoError(t, err)
		err = registry.Register("openai", config2)
		require.NoError(t, err)
		retrieved, err := registry.Get("openai")
		require.NoError(t, err)
		assert.Equal(t, "gpt-4", retrieved.Model)
	})
}

func TestProviderRegistry_Get(t *testing.T) {
	t.Run("Should retrieve registered provider", func(t *testing.T) {
		registry := NewProviderRegistry()
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
			Settings: map[string]string{
				"encoding": "cl100k_base",
			},
		}
		err := registry.Register("openai-gpt4", config)
		require.NoError(t, err)
		retrieved, err := registry.Get("openai-gpt4")
		require.NoError(t, err)
		assert.Equal(t, config.Provider, retrieved.Provider)
		assert.Equal(t, config.Model, retrieved.Model)
		assert.Equal(t, config.APIKey, retrieved.APIKey)
		assert.Equal(t, config.Settings["encoding"], retrieved.Settings["encoding"])
	})
	t.Run("Should fail for non-existent provider", func(t *testing.T) {
		registry := NewProviderRegistry()
		retrieved, err := registry.Get("non-existent")
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "provider 'non-existent' not found")
	})
}

func TestProviderRegistry_List(t *testing.T) {
	t.Run("Should list all registered providers", func(t *testing.T) {
		registry := NewProviderRegistry()
		providers := []string{"openai-gpt4", "anthropic-claude", "google-gemini"}
		for _, name := range providers {
			config := &ProviderConfig{
				Provider: name,
				Model:    "test-model",
			}
			err := registry.Register(name, config)
			require.NoError(t, err)
		}
		listed := registry.List()
		assert.Len(t, listed, 3)
		for _, provider := range providers {
			assert.Contains(t, listed, provider)
		}
	})
	t.Run("Should return empty list for empty registry", func(t *testing.T) {
		registry := NewProviderRegistry()
		listed := registry.List()
		assert.Empty(t, listed)
	})
}

func TestProviderRegistry_Remove(t *testing.T) {
	t.Run("Should remove existing provider", func(t *testing.T) {
		registry := NewProviderRegistry()
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
		}
		err := registry.Register("openai-gpt4", config)
		require.NoError(t, err)
		// Verify it exists
		_, err = registry.Get("openai-gpt4")
		require.NoError(t, err)
		// Remove it
		registry.Remove("openai-gpt4")
		// Verify it's gone
		_, err = registry.Get("openai-gpt4")
		assert.Error(t, err)
		assert.Empty(t, registry.List())
	})
	t.Run("Should not fail for non-existent provider", func(_ *testing.T) {
		registry := NewProviderRegistry()
		// Should not panic or error
		registry.Remove("non-existent")
	})
}

func TestProviderRegistry_RegisterDefaults(t *testing.T) {
	t.Run("Should register all default providers", func(t *testing.T) {
		registry := NewProviderRegistry()
		registry.RegisterDefaults()
		providers := registry.List()
		// Check that we have several default providers
		assert.True(t, len(providers) > 5)
		// Check for specific providers
		expectedProviders := []string{
			"openai-gpt4",
			"openai-gpt4o",
			"openai-gpt35",
			"anthropic-claude",
			"google-gemini-pro",
			"cohere-command-r",
			"deepseek-v2",
		}
		for _, expected := range expectedProviders {
			assert.Contains(t, providers, expected)
		}
	})
	t.Run("Should create valid configurations for defaults", func(t *testing.T) {
		registry := NewProviderRegistry()
		registry.RegisterDefaults()
		// Test a few specific configurations
		openaiConfig, err := registry.Get("openai-gpt4")
		require.NoError(t, err)
		assert.Equal(t, "OpenAI", openaiConfig.Provider)
		assert.Equal(t, "gpt-4", openaiConfig.Model)
		assert.Equal(t, "cl100k_base", openaiConfig.Settings["encoding"])
		anthropicConfig, err := registry.Get("anthropic-claude")
		require.NoError(t, err)
		assert.Equal(t, "Anthropic", anthropicConfig.Provider)
		assert.Equal(t, "claude-3-haiku", anthropicConfig.Model)
		assert.Equal(t, "claude", anthropicConfig.Settings["encoding"])
		googleConfig, err := registry.Get("google-gemini-pro")
		require.NoError(t, err)
		assert.Equal(t, "GoogleAI", googleConfig.Provider)
		assert.Equal(t, "gemini-1.5-pro", googleConfig.Model)
		assert.Equal(t, "gemini", googleConfig.Settings["encoding"])
	})
}

func TestProviderRegistry_Clone(t *testing.T) {
	t.Run("Should create deep copy of provider config", func(t *testing.T) {
		registry := NewProviderRegistry()
		original := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "original-key",
			Endpoint: "https://api.openai.com",
			Settings: map[string]string{
				"encoding": "cl100k_base",
				"timeout":  "30s",
			},
		}
		err := registry.Register("openai-gpt4", original)
		require.NoError(t, err)
		cloned, err := registry.Clone("openai-gpt4")
		require.NoError(t, err)
		// Verify all fields are copied
		assert.Equal(t, original.Provider, cloned.Provider)
		assert.Equal(t, original.Model, cloned.Model)
		assert.Equal(t, original.APIKey, cloned.APIKey)
		assert.Equal(t, original.Endpoint, cloned.Endpoint)
		assert.Equal(t, original.Settings["encoding"], cloned.Settings["encoding"])
		assert.Equal(t, original.Settings["timeout"], cloned.Settings["timeout"])
		// Verify it's a deep copy (modifying cloned doesn't affect original)
		cloned.APIKey = "new-key"
		cloned.Settings["encoding"] = "new-encoding"
		retrieved, err := registry.Get("openai-gpt4")
		require.NoError(t, err)
		assert.Equal(t, "original-key", retrieved.APIKey)
		assert.Equal(t, "cl100k_base", retrieved.Settings["encoding"])
	})
	t.Run("Should fail for non-existent provider", func(t *testing.T) {
		registry := NewProviderRegistry()
		cloned, err := registry.Clone("non-existent")
		assert.Error(t, err)
		assert.Nil(t, cloned)
		assert.Contains(t, err.Error(), "provider 'non-existent' not found")
	})
}

func TestProviderRegistry_ThreadSafety(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		registry := NewProviderRegistry()
		// Populate with some initial data
		registry.RegisterDefaults()
		// Run multiple goroutines concurrently
		done := make(chan bool, 20)
		// Readers
		for i := 0; i < 10; i++ {
			go func(_ int) {
				providers := registry.List()
				assert.True(t, len(providers) > 0)
				// Try to get a provider - handle race condition where provider might be removed
				if len(providers) > 0 {
					config, err := registry.Get(providers[0])
					// Either success or provider not found (due to concurrent removal) is acceptable
					if err == nil {
						assert.NotNil(t, config)
					} else {
						assert.Contains(t, err.Error(), "not found")
					}
				}
				done <- true
			}(i)
		}
		// Writers
		for i := 0; i < 10; i++ {
			go func(i int) {
				config := &ProviderConfig{
					Provider: "TestProvider",
					Model:    "test-model",
				}
				name := fmt.Sprintf("test-provider-%d", i)
				err := registry.Register(name, config)
				assert.NoError(t, err)
				// Try to clone it
				cloned, err := registry.Clone(name)
				assert.NoError(t, err)
				assert.NotNil(t, cloned)
				// Remove it
				registry.Remove(name)
				done <- true
			}(i)
		}
		// Wait for all goroutines to complete
		for i := 0; i < 20; i++ {
			<-done
		}
	})
}
