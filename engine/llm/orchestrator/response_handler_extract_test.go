package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTopLevelErrorMessage_Variants(t *testing.T) {
	t.Run("Map_with_message_field", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"message":"abc"}}`)
		assert.True(t, ok)
		assert.Equal(t, "abc", msg)
	})
	t.Run("Map_without_message_field_serializes", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error":{"code":1}}`)
		assert.True(t, ok)
		assert.Contains(t, msg, "code")
	})
	t.Run("Non_string_error_value_number", func(t *testing.T) {
		msg, ok := extractTopLevelErrorMessage(`{"error": 123}`)
		assert.True(t, ok)
		assert.Equal(t, "123", msg)
	})
}
