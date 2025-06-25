package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

func TestConfigResolverPatternIntegration(t *testing.T) {
	log := logger.NewForTests()
	t.Run("Should validate and use regex patterns directly", func(t *testing.T) {
		mm := &Manager{
			log: log,
		}
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with privacy patterns",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`,                               // SSN pattern
					`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email pattern
					`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{3,6}\b`,        // Credit card pattern
				},
			},
		}
		resource, err := mm.configToResource(config)
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		// Should have all patterns
		assert.Len(t, resource.PrivacyPolicy.RedactPatterns, 3)
		// Check that patterns are valid regex
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[0], "123-45-6789")         // SSN
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[1], "test@example.com")    // Email
		assert.Regexp(t, resource.PrivacyPolicy.RedactPatterns[2], "4111 1111 1111 1111") // Credit card
	})
	t.Run("Should reject invalid regex patterns", func(t *testing.T) {
		mm := &Manager{
			log: log,
		}
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with invalid pattern",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`, // Valid SSN pattern
					`[invalid(`,             // Invalid regex
				},
			},
		}
		resource, err := mm.configToResource(config)
		assert.Error(t, err, "Should return error for invalid patterns")
		assert.Nil(t, resource)
	})
	t.Run("Should reject ReDoS vulnerable patterns", func(t *testing.T) {
		mm := &Manager{
			log: log,
		}
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with dangerous pattern",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`, // Valid SSN pattern
					`(a+)+`,                 // ReDoS vulnerable pattern
				},
			},
		}
		resource, err := mm.configToResource(config)
		assert.Error(t, err, "Should return error for dangerous patterns")
		assert.Nil(t, resource)
	})
	t.Run("Should preserve other privacy policy settings", func(t *testing.T) {
		mm := &Manager{
			log: log,
		}
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory with full privacy policy",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns: []string{
					`\b\d{3}-\d{2}-\d{4}\b`,
					`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
				},
				NonPersistableMessageTypes: []string{"system", "tool"},
				DefaultRedactionString:     "[HIDDEN]",
			},
		}
		resource, err := mm.configToResource(config)
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		// Check all settings are preserved
		assert.Equal(t, []string{"system", "tool"}, resource.PrivacyPolicy.NonPersistableMessageTypes)
		assert.Equal(t, "[HIDDEN]", resource.PrivacyPolicy.DefaultRedactionString)
		assert.Len(t, resource.PrivacyPolicy.RedactPatterns, 2)
	})
	t.Run("Should handle empty patterns list", func(t *testing.T) {
		mm := &Manager{
			log: log,
		}
		config := &Config{
			ID:          "test-memory",
			Description: "Test memory without patterns",
			Type:        "token_based",
			MaxTokens:   1000,
			PrivacyPolicy: &memcore.PrivacyPolicyConfig{
				RedactPatterns:             []string{},
				NonPersistableMessageTypes: []string{"system"},
			},
		}
		resource, err := mm.configToResource(config)
		require.NoError(t, err)
		require.NotNil(t, resource)
		require.NotNil(t, resource.PrivacyPolicy)
		assert.Empty(t, resource.PrivacyPolicy.RedactPatterns)
		assert.Equal(t, []string{"system"}, resource.PrivacyPolicy.NonPersistableMessageTypes)
	})
}
