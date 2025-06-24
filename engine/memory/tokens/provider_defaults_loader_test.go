package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProviderDefaults(t *testing.T) {
	t.Run("Should load provider defaults from embedded YAML", func(t *testing.T) {
		defaults, err := LoadProviderDefaults()
		require.NoError(t, err)
		assert.NotNil(t, defaults)
		// Check that we have the expected number of providers
		assert.GreaterOrEqual(t, len(defaults.Providers), 10)
		// Check specific providers exist
		providerNames := make(map[string]bool)
		for _, p := range defaults.Providers {
			providerNames[p.Name] = true
		}
		assert.True(t, providerNames["openai-gpt4"])
		assert.True(t, providerNames["anthropic-claude"])
		assert.True(t, providerNames["google-gemini-pro"])
		assert.True(t, providerNames["deepseek-v2"])
		// Check a specific provider configuration
		for _, p := range defaults.Providers {
			if p.Name == "openai-gpt4" {
				assert.Equal(t, "OpenAI", p.Provider)
				assert.Equal(t, "gpt-4", p.Model)
				assert.Equal(t, "cl100k_base", p.Settings["encoding"])
				break
			}
		}
	})
}

func TestProviderRegistry_RegisterDefaultsFromYAML(t *testing.T) {
	t.Run("Should register all defaults from YAML", func(t *testing.T) {
		registry := NewProviderRegistry()
		err := registry.RegisterDefaultsFromYAML()
		require.NoError(t, err)
		// Check that providers were registered
		providers := registry.List()
		assert.GreaterOrEqual(t, len(providers), 10)
		// Check specific provider can be retrieved
		config, err := registry.Get("openai-gpt4")
		require.NoError(t, err)
		assert.Equal(t, "OpenAI", config.Provider)
		assert.Equal(t, "gpt-4", config.Model)
		// Check another provider
		config, err = registry.Get("anthropic-claude-sonnet")
		require.NoError(t, err)
		assert.Equal(t, "Anthropic", config.Provider)
		assert.Equal(t, "claude-3-sonnet", config.Model)
	})
}
