package memory

import (
	"testing"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_FromMap_ProjectStandard(t *testing.T) {
	// Test that FromMap properly uses mapstructure tags to map fields
	t.Run("Should map fields using mapstructure tags", func(t *testing.T) {
		// Simulate the exact structure that autoloader provides
		yamlMap := map[string]any{
			"resource":     "memory",
			"id":           "user_memory",
			"description":  "User conversation history",
			"version":      "0.1.0",
			"type":         "token_based",
			"max_tokens":   2000,
			"max_messages": 50,
			"flushing": map[string]any{
				"type":      "simple_fifo",
				"threshold": 0.8,
			},
			"persistence": map[string]any{
				"ttl": "168h",
			},
			"privacy_policy": map[string]any{
				"redact_patterns":          []string{"\\b\\d{3}-\\d{2}-\\d{4}\\b"},
				"default_redaction_string": "[REDACTED]",
			},
		}

		// Create config and convert from map
		config := &Config{}
		err := config.FromMap(yamlMap)
		require.NoError(t, err)

		// Verify all fields were properly mapped
		assert.Equal(t, "memory", config.Resource)
		assert.Equal(t, "user_memory", config.ID)
		assert.Equal(t, "User conversation history", config.Description)
		assert.Equal(t, "0.1.0", config.Version)
		assert.Equal(t, memcore.TokenBasedMemory, config.Type)
		assert.Equal(t, 2000, config.MaxTokens)
		assert.Equal(t, 50, config.MaxMessages)

		// Verify nested structures
		require.NotNil(t, config.Flushing)
		assert.Equal(t, memcore.SimpleFIFOFlushing, config.Flushing.Type)

		assert.Equal(t, "168h", config.Persistence.TTL)

		require.NotNil(t, config.PrivacyPolicy)
		assert.Len(t, config.PrivacyPolicy.RedactPatterns, 1)
		assert.Equal(t, "[REDACTED]", config.PrivacyPolicy.DefaultRedactionString)
	})

	t.Run("Should validate after FromMap", func(t *testing.T) {
		yamlMap := map[string]any{
			"resource":   "memory",
			"id":         "test_memory",
			"type":       "token_based",
			"max_tokens": 1000,
			"persistence": map[string]any{
				"ttl": "24h",
			},
		}

		config := &Config{}
		err := config.FromMap(yamlMap)
		require.NoError(t, err)

		// Validate should pass
		err = config.Validate()
		assert.NoError(t, err)
	})
}
