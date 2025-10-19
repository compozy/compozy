package tokens

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memcore "github.com/compozy/compozy/engine/memory/core"
)

func TestAPIKeyResolver_ResolveAPIKey(t *testing.T) {
	resolver := NewAPIKeyResolver()
	// Set up test environment variables
	os.Setenv("TEST_API_KEY", "test-key-from-env")
	os.Setenv("OPENAI_API_KEY", "openai-test-key")
	defer func() {
		os.Unsetenv("TEST_API_KEY")
		os.Unsetenv("OPENAI_API_KEY")
	}()
	t.Run("Should resolve from APIKeyEnv field", func(t *testing.T) {
		config := &memcore.TokenProviderConfig{
			Provider:  "TestProvider",
			Model:     "test-model",
			APIKeyEnv: "TEST_API_KEY",
			APIKey:    "direct-key", // Should be ignored
		}
		key := resolver.ResolveAPIKey(t.Context(), config)
		assert.Equal(t, "test-key-from-env", key)
	})
	t.Run("Should resolve from inline env var reference", func(t *testing.T) {
		config := &memcore.TokenProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "${OPENAI_API_KEY}",
		}
		key := resolver.ResolveAPIKey(t.Context(), config)
		assert.Equal(t, "openai-test-key", key)
	})
	t.Run("Should use direct value when no env var", func(t *testing.T) {
		config := &memcore.TokenProviderConfig{
			Provider: "TestProvider",
			Model:    "test-model",
			APIKey:   "direct-api-key",
		}
		key := resolver.ResolveAPIKey(t.Context(), config)
		assert.Equal(t, "direct-api-key", key)
	})
	t.Run("Should return empty string when env var not set", func(t *testing.T) {
		config := &memcore.TokenProviderConfig{
			Provider:  "TestProvider",
			Model:     "test-model",
			APIKeyEnv: "NONEXISTENT_KEY",
		}
		key := resolver.ResolveAPIKey(t.Context(), config)
		assert.Equal(t, "", key)
	})
}

func TestAPIKeyResolver_ResolveProviderConfig(t *testing.T) {
	resolver := NewAPIKeyResolver()
	// Set up test environment
	os.Setenv("ANTHROPIC_API_KEY", "anthropic-test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	t.Run("Should create resolved provider config", func(t *testing.T) {
		config := &memcore.TokenProviderConfig{
			Provider:  "Anthropic",
			Model:     "claude-3",
			APIKeyEnv: "ANTHROPIC_API_KEY",
			Endpoint:  "https://api.anthropic.com",
			Settings: map[string]string{
				"encoding": "claude",
			},
		}
		resolved := resolver.ResolveProviderConfig(t.Context(), config)
		assert.Equal(t, "Anthropic", resolved.Provider)
		assert.Equal(t, "claude-3", resolved.Model)
		assert.Equal(t, "anthropic-test-key", resolved.APIKey)
		assert.Equal(t, "https://api.anthropic.com", resolved.Endpoint)
		assert.Equal(t, "claude", resolved.Settings["encoding"])
	})
}

func TestGetRequiredEnvVars(t *testing.T) {
	tests := []struct {
		provider string
		expected []string
	}{
		{"openai", []string{"OPENAI_API_KEY"}},
		{"anthropic", []string{"ANTHROPIC_API_KEY"}},
		{"googleai", []string{"GOOGLE_API_KEY"}},
		{"google", []string{"GOOGLE_API_KEY"}},
		{"cohere", []string{"COHERE_API_KEY"}},
		{"deepseek", []string{"DEEPSEEK_API_KEY"}},
		{"custom", []string{"CUSTOM_API_KEY"}},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			vars := GetRequiredEnvVars(tt.provider)
			require.Equal(t, tt.expected, vars)
		})
	}
}
