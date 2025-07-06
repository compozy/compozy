package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractWorkflowExecID(t *testing.T) {
	t.Run("Should extract workflow exec ID from nested structure", func(t *testing.T) {
		contextData := map[string]any{
			"workflow": map[string]any{
				"exec_id": "test-exec-123",
				"name":    "test-workflow",
			},
			"other_data": "value",
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "test-exec-123", result)
	})

	t.Run("Should return unknown when workflow is not a map", func(t *testing.T) {
		contextData := map[string]any{
			"workflow": "not-a-map",
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when exec_id is not a string", func(t *testing.T) {
		contextData := map[string]any{
			"workflow": map[string]any{
				"exec_id": 123, // number instead of string
			},
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when exec_id is empty", func(t *testing.T) {
		contextData := map[string]any{
			"workflow": map[string]any{
				"exec_id": "",
			},
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when workflow key is missing", func(t *testing.T) {
		contextData := map[string]any{
			"other_data": "value",
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when exec_id key is missing", func(t *testing.T) {
		contextData := map[string]any{
			"workflow": map[string]any{
				"name": "test-workflow",
			},
		}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when contextData is nil", func(t *testing.T) {
		result := ExtractWorkflowExecID(nil)
		assert.Equal(t, "unknown", result)
	})

	t.Run("Should return unknown when contextData is empty", func(t *testing.T) {
		contextData := map[string]any{}

		result := ExtractWorkflowExecID(contextData)
		assert.Equal(t, "unknown", result)
	})
}
