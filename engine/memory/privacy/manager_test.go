package privacy_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/privacy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivacyManager(t *testing.T) {
	t.Run("Should register and retrieve privacy policy", func(t *testing.T) {
		pm := privacy.NewManager()
		require.NotNil(t, pm)

		policy := &core.PrivacyPolicyConfig{
			RedactPatterns:         []string{`\b\d{3}-\d{2}-\d{4}\b`}, // SSN pattern
			DefaultRedactionString: "[SSN]",
		}

		err := pm.RegisterPolicy("test-resource", policy)
		assert.NoError(t, err)

		retrievedPolicy, exists := pm.GetPolicy("test-resource")
		assert.True(t, exists)
		assert.Equal(t, policy, retrievedPolicy)
	})

	t.Run("Should reject dangerous regex patterns", func(t *testing.T) {
		pm := privacy.NewManager()

		dangerousPatterns := []string{
			`(a+)+`,
			`(a*)*`,
			`(.+)+`,
			`(\S+)+`,
		}

		for _, pattern := range dangerousPatterns {
			policy := &core.PrivacyPolicyConfig{
				RedactPatterns: []string{pattern},
			}
			err := pm.RegisterPolicy("test-resource", policy)
			assert.Error(t, err, "Pattern %s should be rejected", pattern)
			assert.Contains(t, err.Error(), "unsafe regex pattern")
		}
	})

	t.Run("Should apply privacy controls correctly", func(t *testing.T) {
		pm := privacy.NewManager()

		policy := &core.PrivacyPolicyConfig{
			RedactPatterns:         []string{`\b\d{3}-\d{2}-\d{4}\b`}, // SSN pattern
			DefaultRedactionString: "[SSN]",
		}

		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)

		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "My SSN is 123-45-6789",
		}
		metadata := core.PrivacyMetadata{}

		redactedMsg, updatedMetadata, err := pm.ApplyPrivacyControls(
			context.Background(),
			msg,
			"test-resource",
			metadata,
		)

		assert.NoError(t, err)
		assert.Equal(t, "My SSN is [SSN]", redactedMsg.Content)
		assert.True(t, updatedMetadata.RedactionApplied)
		assert.False(t, updatedMetadata.DoNotPersist)
	})

	t.Run("Should handle non-persistable message types", func(t *testing.T) {
		pm := privacy.NewManager()

		policy := &core.PrivacyPolicyConfig{
			NonPersistableMessageTypes: []string{"system", "debug"},
		}

		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)

		// Test persistable message
		assert.True(t, pm.ShouldPersistMessage("user", policy.NonPersistableMessageTypes))
		assert.True(t, pm.ShouldPersistMessage("assistant", policy.NonPersistableMessageTypes))

		// Test non-persistable messages
		assert.False(t, pm.ShouldPersistMessage("system", policy.NonPersistableMessageTypes))
		assert.False(t, pm.ShouldPersistMessage("debug", policy.NonPersistableMessageTypes))
		assert.False(t, pm.ShouldPersistMessage("SYSTEM", policy.NonPersistableMessageTypes)) // Case insensitive
	})

	t.Run("Should handle circuit breaker", func(t *testing.T) {
		pm := privacy.NewManager()

		// Initially circuit breaker should be closed
		isOpen, consecutiveErrors, maxErrors := pm.GetCircuitBreakerStatus()
		assert.False(t, isOpen)
		assert.Equal(t, 0, consecutiveErrors)
		assert.Equal(t, 10, maxErrors)

		// Reset circuit breaker
		pm.ResetCircuitBreaker()
		isOpen, consecutiveErrors, _ = pm.GetCircuitBreakerStatus()
		assert.False(t, isOpen)
		assert.Equal(t, 0, consecutiveErrors)
	})

	t.Run("Should handle redact content directly", func(t *testing.T) {
		pm := privacy.NewManager()

		content := "My phone is 555-1234 and email is test@example.com"
		patterns := []string{
			`\b\d{3}-\d{4}\b`, // Phone pattern
			`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email pattern
		}

		redacted, err := pm.RedactContent(content, patterns, "[REDACTED]")
		assert.NoError(t, err)
		assert.Equal(t, "My phone is [REDACTED] and email is [REDACTED]", redacted)
	})
}
