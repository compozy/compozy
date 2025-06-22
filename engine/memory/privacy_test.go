package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivacyManager_RegisterPolicy(t *testing.T) {
	t.Run("Should register valid privacy policy", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			RedactPatterns:             []string{`\b\d{3}-\d{2}-\d{4}\b`}, // SSN pattern
			NonPersistableMessageTypes: []string{"system"},
			DefaultRedactionString:     "[REDACTED]",
		}
		err := pm.RegisterPolicy("test-resource", policy)
		assert.NoError(t, err)
		// Verify policy was registered
		pm.mu.RLock()
		defer pm.mu.RUnlock()
		assert.NotNil(t, pm.policies["test-resource"])
		assert.Len(t, pm.compiledPatterns["test-resource"], 1)
	})
	t.Run("Should handle empty resource ID", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{}
		err := pm.RegisterPolicy("", policy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource ID cannot be empty")
	})
	t.Run("Should handle nil policy", func(t *testing.T) {
		pm := NewPrivacyManager()
		err := pm.RegisterPolicy("test-resource", nil)
		assert.NoError(t, err)
	})
	t.Run("Should handle invalid regex pattern", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			RedactPatterns: []string{`[invalid regex`},
		}
		err := pm.RegisterPolicy("test-resource", policy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
}

func TestPrivacyManager_RedactMessage(t *testing.T) {
	t.Run("Should redact SSN from message", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			RedactPatterns:         []string{`\b\d{3}-\d{2}-\d{4}\b`},
			DefaultRedactionString: "[SSN]",
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "My SSN is 123-45-6789 please keep it safe",
		}
		redacted, err := pm.RedactMessage(context.Background(), "test-resource", msg)
		assert.NoError(t, err)
		assert.Equal(t, "My SSN is [SSN] please keep it safe", redacted.Content)
		assert.Equal(t, llm.MessageRoleUser, redacted.Role)
	})
	t.Run("Should redact multiple patterns", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			RedactPatterns: []string{
				`\b\d{3}-\d{2}-\d{4}\b`,                      // SSN
				`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, // Credit card
			},
			DefaultRedactionString: "[REDACTED]",
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "SSN: 123-45-6789, Card: 1234 5678 9012 3456",
		}
		redacted, err := pm.RedactMessage(context.Background(), "test-resource", msg)
		assert.NoError(t, err)
		assert.Equal(t, "SSN: [REDACTED], Card: [REDACTED]", redacted.Content)
	})
	t.Run("Should handle no policy for resource", func(t *testing.T) {
		pm := NewPrivacyManager()
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "My SSN is 123-45-6789",
		}
		redacted, err := pm.RedactMessage(context.Background(), "unknown-resource", msg)
		assert.NoError(t, err)
		assert.Equal(t, msg.Content, redacted.Content) // No redaction
	})
	t.Run("Should use default redaction string", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			RedactPatterns: []string{`\b\d{3}-\d{2}-\d{4}\b`},
			// No DefaultRedactionString specified
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "SSN: 123-45-6789",
		}
		redacted, err := pm.RedactMessage(context.Background(), "test-resource", msg)
		assert.NoError(t, err)
		assert.Equal(t, "SSN: [REDACTED]", redacted.Content)
	})
}

func TestPrivacyManager_ShouldPersistMessage(t *testing.T) {
	t.Run("Should not persist non-persistable message types", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			NonPersistableMessageTypes: []string{"system", "tool"},
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		// Test system message
		msg := llm.Message{Role: llm.MessageRoleSystem, Content: "System prompt"}
		assert.False(t, pm.ShouldPersistMessage("test-resource", msg))
		// Test tool message
		msg = llm.Message{Role: llm.MessageRoleTool, Content: "Tool response"}
		assert.False(t, pm.ShouldPersistMessage("test-resource", msg))
		// Test user message (should persist)
		msg = llm.Message{Role: llm.MessageRoleUser, Content: "User message"}
		assert.True(t, pm.ShouldPersistMessage("test-resource", msg))
	})
	t.Run("Should persist all messages when no policy", func(t *testing.T) {
		pm := NewPrivacyManager()
		msg := llm.Message{Role: llm.MessageRoleSystem, Content: "System prompt"}
		assert.True(t, pm.ShouldPersistMessage("unknown-resource", msg))
	})
	t.Run("Should handle case-insensitive role matching", func(t *testing.T) {
		pm := NewPrivacyManager()
		policy := &PrivacyPolicyConfig{
			NonPersistableMessageTypes: []string{"SYSTEM"},
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		msg := llm.Message{Role: "system", Content: "System prompt"}
		assert.False(t, pm.ShouldPersistMessage("test-resource", msg))
	})
}

func TestPrivacyManager_CircuitBreaker(t *testing.T) {
	t.Run("Should handle circuit breaker when errors exceed threshold", func(t *testing.T) {
		pm := NewPrivacyManager()
		// Register a policy so we enter the redaction logic
		policy := &PrivacyPolicyConfig{
			RedactPatterns: []string{`test`},
		}
		err := pm.RegisterPolicy("test-resource", policy)
		require.NoError(t, err)
		pm.consecutiveErrors = pm.maxConsecutiveErrors // Simulate max errors reached
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "Test"}
		_, err = pm.RedactMessage(context.Background(), "test-resource", msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker open")
	})
}

func TestValidatePrivacyPolicy(t *testing.T) {
	t.Run("Should validate valid policy", func(t *testing.T) {
		policy := &PrivacyPolicyConfig{
			RedactPatterns:             []string{`\b\d{3}-\d{2}-\d{4}\b`, `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`},
			NonPersistableMessageTypes: []string{"system"},
			DefaultRedactionString:     "[REDACTED]",
		}
		err := ValidatePrivacyPolicy(policy)
		assert.NoError(t, err)
	})
	t.Run("Should handle nil policy", func(t *testing.T) {
		err := ValidatePrivacyPolicy(nil)
		assert.NoError(t, err)
	})
	t.Run("Should reject invalid regex", func(t *testing.T) {
		policy := &PrivacyPolicyConfig{
			RedactPatterns: []string{`[invalid`},
		}
		err := ValidatePrivacyPolicy(policy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
}

func TestBuildRedactionPattern(t *testing.T) {
	t.Run("Should build patterns from common patterns", func(t *testing.T) {
		patterns := BuildRedactionPattern("ssn", "credit_card", "email")
		assert.Len(t, patterns, 3)
		assert.Equal(t, `\b\d{3}-\d{2}-\d{4}\b`, patterns[0])
		assert.Equal(t, `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, patterns[1])
		assert.Equal(t, `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, patterns[2])
	})
	t.Run("Should handle custom patterns", func(t *testing.T) {
		patterns := BuildRedactionPattern("ssn", `custom-\d+`)
		assert.Len(t, patterns, 2)
		assert.Equal(t, `\b\d{3}-\d{2}-\d{4}\b`, patterns[0])
		assert.Equal(t, `custom-\d+`, patterns[1])
	})
}
