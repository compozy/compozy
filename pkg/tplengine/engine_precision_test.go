package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		// Test large integer precision preservation
		result1, err := engine.ParseAny("9007199254740992", context)
		require.NoError(t, err)
		assert.Equal(t, "9007199254740992", result1, "Large integers should be preserved as strings")

		// Test high precision decimal preservation
		result2, err := engine.ParseAny("0.123456789123456789", context)
		require.NoError(t, err)
		assert.Equal(t, "0.123456789123456789", result2, "High precision decimals should be preserved as strings")

		// Test normal integer conversion
		result3, err := engine.ParseAny("42", context)
		require.NoError(t, err)
		assert.Equal(t, int64(42), result3, "Normal integers should be converted to int64")

		// Test normal float conversion
		result4, err := engine.ParseAny("3.14", context)
		require.NoError(t, err)
		assert.Equal(t, float64(3.14), result4, "Normal floats should be converted to float64")
	})

	t.Run("Should apply precision to template results", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(true)

		context := map[string]any{
			"large_int": "9007199254740992",
			"decimal":   "0.123456789123456789",
		}

		// Test template rendering preserves precision for large values
		result1, err := engine.ParseAny("{{ .large_int }}", context)
		require.NoError(t, err)
		assert.Equal(t, "9007199254740992", result1, "Template rendering should preserve large integer precision")

		// Test template rendering preserves precision for high-precision decimals
		result2, err := engine.ParseAny("{{ .decimal }}", context)
		require.NoError(t, err)
		assert.Equal(t, "0.123456789123456789", result2, "Template rendering should preserve decimal precision")
	})

	t.Run("Should not apply precision when disabled", func(t *testing.T) {
		engine := NewEngine(FormatText).WithPrecisionPreservation(false)

		context := map[string]any{
			"value": "123",
		}

		// Test disabled precision preservation behavior
		result, err := engine.ParseAny("123", context)
		require.NoError(t, err)
		assert.Equal(
			t,
			"123",
			result,
			"Numeric strings should remain as strings when precision preservation is disabled",
		)
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
		require.NoError(t, err)

		m, ok := result.(map[string]any)
		require.True(t, ok, "Result should be a map")

		// Verify precision preservation behavior for different value types
		assert.Equal(t, "9007199254740992", m["large_int"], "Large integers should be preserved as strings")
		assert.Equal(t, "0.123456789123456789", m["decimal"], "High precision decimals should be preserved as strings")
		assert.Equal(t, int64(42), m["normal"], "Normal integers should be converted to int64")
		assert.Equal(t, "hello", m["text"], "Text values should remain unchanged")
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
		require.NoError(t, err)

		arr, ok := result.([]any)
		require.True(t, ok, "Result should be an array")

		// Verify precision preservation behavior for array elements
		assert.Equal(t, "9007199254740992", arr[0], "Large integers in arrays should be preserved as strings")
		assert.Equal(
			t,
			"0.123456789123456789",
			arr[1],
			"High precision decimals in arrays should be preserved as strings",
		)
		assert.Equal(t, int64(42), arr[2], "Normal integers in arrays should be converted to int64")
		assert.Equal(t, "hello", arr[3], "Text values in arrays should remain unchanged")
	})
}
