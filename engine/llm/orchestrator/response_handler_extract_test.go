package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTopLevelErrorMessage_Variants(t *testing.T) {
	t.Run("Should extract message field", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"message":"abc"}}`)
		assert.True(t, ok)
		assert.Equal(t, "abc", msg)
	})
	t.Run("Should serialize map without message field", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"code":1}}`)
		assert.True(t, ok)
		assert.Contains(t, msg, "code")
	})
	t.Run("Should stringify numeric top-level error value", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error": 123}`)
		assert.True(t, ok)
		assert.Equal(t, "123", msg)
	})
}
