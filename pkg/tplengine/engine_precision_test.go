package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateEngine_PrecisionPreservation(t *testing.T) {
	t.Run("Should preserve numeric precision when enabled", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(true)

		context := map[string]any{
			"large_int":    "9007199254740992",
			"decimal":      "0.123456789123456789",
			"normal_int":   "42",
			"normal_float": "3.14",
		}

		// Test direct value parsing (no template markers)
		result1, err := engine.ParseAny("9007199254740992", context)
		assert.NoError(t, err)
		assert.Equal(t, "9007199254740992", result1) // Preserved as string

		result2, err := engine.ParseAny("0.123456789123456789", context)
		assert.NoError(t, err)
		assert.Equal(t, "0.123456789123456789", result2) // Preserved as string

		result3, err := engine.ParseAny("42", context)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), result3) // Converted to int64

		result4, err := engine.ParseAny("3.14", context)
		assert.NoError(t, err)
		assert.Equal(t, float64(3.14), result4) // Converted to float64
	})

	t.Run("Should apply precision to template results", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(true)

		context := map[string]any{
			"large_int": "9007199254740992",
			"decimal":   "0.123456789123456789",
		}

		// Test template rendering with precision
		result1, err := engine.ParseAny("{{ .large_int }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "9007199254740992", result1) // Preserved as string

		result2, err := engine.ParseAny("{{ .decimal }}", context)
		assert.NoError(t, err)
		assert.Equal(t, "0.123456789123456789", result2) // Preserved as string
	})

	t.Run("Should not apply precision when disabled", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(false)

		context := map[string]any{
			"value": "123",
		}

		// Without precision preservation, strings remain strings
		result, err := engine.ParseAny("123", context)
		assert.NoError(t, err)
		assert.Equal(t, "123", result) // Remains as string
	})

	t.Run("Should handle maps with precision", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(true)

		input := map[string]any{
			"large_int": "9007199254740992",
			"decimal":   "0.123456789123456789",
			"normal":    "42",
			"text":      "hello",
		}

		result, err := engine.ParseAny(input, nil)
		assert.NoError(t, err)

		m, ok := result.(map[string]any)
		assert.True(t, ok)

		assert.Equal(t, "9007199254740992", m["large_int"])
		assert.Equal(t, "0.123456789123456789", m["decimal"])
		assert.Equal(t, int64(42), m["normal"])
		assert.Equal(t, "hello", m["text"])
	})

	t.Run("Should handle arrays with precision", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(true)

		input := []any{
			"9007199254740992",
			"0.123456789123456789",
			"42",
			"hello",
		}

		result, err := engine.ParseAny(input, nil)
		assert.NoError(t, err)

		arr, ok := result.([]any)
		assert.True(t, ok)

		assert.Equal(t, "9007199254740992", arr[0])
		assert.Equal(t, "0.123456789123456789", arr[1])
		assert.Equal(t, int64(42), arr[2])
		assert.Equal(t, "hello", arr[3])
	})
}
