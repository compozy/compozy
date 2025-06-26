package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestType_String(t *testing.T) {
	assert.Equal(t, "token_based", TokenBasedMemory.String())
	assert.Equal(t, "message_count_based", MessageCountBasedMemory.String())
	assert.Equal(t, "buffer", BufferMemory.String())
}

func TestResource_Validate(t *testing.T) {
	t.Run("Valid token-based resource", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
		}
		assert.NoError(t, resource.Validate())
	})

	t.Run("Empty ID should fail", func(t *testing.T) {
		resource := &Resource{
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
		}
		assert.Error(t, resource.Validate())
	})

	t.Run("Token-based without limits should fail", func(t *testing.T) {
		resource := &Resource{
			ID:   "test-memory",
			Type: TokenBasedMemory,
		}
		assert.Error(t, resource.Validate())
	})
	t.Run("Valid TTL formats should pass", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
			AppendTTL: "5m",
			ClearTTL:  "1h30m",
			FlushTTL:  "2s",
		}
		assert.NoError(t, resource.Validate())
	})
	t.Run("Invalid AppendTTL format should fail", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
			AppendTTL: "invalid",
		}
		err := resource.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid AppendTTL format")
	})
	t.Run("Invalid ClearTTL format should fail", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
			ClearTTL:  "5 minutes",
		}
		err := resource.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ClearTTL format")
	})
	t.Run("Invalid FlushTTL format should fail", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
			FlushTTL:  "2hrs",
		}
		err := resource.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid FlushTTL format")
	})
	t.Run("Empty TTL strings should pass", func(t *testing.T) {
		resource := &Resource{
			ID:        "test-memory",
			Type:      TokenBasedMemory,
			MaxTokens: 1000,
			AppendTTL: "",
			ClearTTL:  "",
			FlushTTL:  "",
		}
		assert.NoError(t, resource.Validate())
	})
}

func TestResource_GetEffectiveMaxTokens(t *testing.T) {
	t.Run("Uses MaxTokens when set", func(t *testing.T) {
		resource := &Resource{MaxTokens: 1000}
		assert.Equal(t, 1000, resource.GetEffectiveMaxTokens())
	})

	t.Run("Uses MaxContextRatio when MaxTokens is 0", func(t *testing.T) {
		resource := &Resource{
			MaxContextRatio:  0.8,
			ModelContextSize: 4096,
		}
		assert.Equal(t, 3276, resource.GetEffectiveMaxTokens())
	})

	t.Run("Falls back to default", func(t *testing.T) {
		resource := &Resource{}
		assert.Equal(t, DefaultMaxTokens, resource.GetEffectiveMaxTokens())
	})
}
