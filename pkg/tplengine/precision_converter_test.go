package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrecisionConverter_ConvertWithPrecision(t *testing.T) {
	t.Run("Should preserve large integers that exceed int64", func(t *testing.T) {
		pc := NewPrecisionConverter()

		tests := []struct {
			name     string
			input    string
			expected any
		}{
			{
				name:     "MAX_SAFE_INTEGER + 1",
				input:    "9007199254740992",
				expected: "9007199254740992", // Preserved as string
			},
			{
				name:     "Very large integer",
				input:    "123456789012345678901234567890",
				expected: "123456789012345678901234567890", // Preserved as string
			},
			{
				name:     "Normal int64",
				input:    "123456789",
				expected: int64(123456789),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := pc.ConvertWithPrecision(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("Should preserve high-precision decimals", func(t *testing.T) {
		pc := NewPrecisionConverter()

		tests := []struct {
			name     string
			input    string
			expected any
		}{
			{
				name:     "High precision decimal",
				input:    "0.123456789123456789",
				expected: "0.123456789123456789", // Preserved as string
			},
			{
				name:     "Normal float64",
				input:    "123.456",
				expected: float64(123.456),
			},
			{
				name:     "Scientific notation",
				input:    "1.23e-10",
				expected: float64(1.23e-10),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := pc.ConvertWithPrecision(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("Should handle non-numeric strings", func(t *testing.T) {
		pc := NewPrecisionConverter()

		tests := []struct {
			name     string
			input    string
			expected any
		}{
			{
				name:     "Text string",
				input:    "hello world",
				expected: "hello world",
			},
			{
				name:     "Empty string",
				input:    "",
				expected: "",
			},
			{
				name:     "Whitespace",
				input:    "   ",
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := pc.ConvertWithPrecision(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestPrecisionConverter_ConvertJSONWithPrecision(t *testing.T) {
	t.Run("Should parse JSON with numeric precision", func(t *testing.T) {
		pc := NewPrecisionConverter()

		jsonStr := `{
			"large_int": 9007199254740992,
			"high_precision": 0.123456789123456789,
			"normal_int": 42,
			"normal_float": 3.14,
			"text": "hello"
		}`

		result, err := pc.ConvertJSONWithPrecision(jsonStr)
		assert.NoError(t, err)

		m, ok := result.(map[string]any)
		assert.True(t, ok)

		// Large integer preserved as string
		assert.Equal(t, "9007199254740992", m["large_int"])

		// High precision decimal preserved as string
		assert.Equal(t, "0.123456789123456789", m["high_precision"])

		// Normal numbers converted to appropriate types
		assert.Equal(t, int64(42), m["normal_int"])
		assert.Equal(t, float64(3.14), m["normal_float"])

		// Non-numeric values unchanged
		assert.Equal(t, "hello", m["text"])
	})

	t.Run("Should handle nested structures", func(t *testing.T) {
		pc := NewPrecisionConverter()

		jsonStr := `{
			"data": {
				"items": [
					{"value": 9007199254740992},
					{"value": 123}
				]
			}
		}`

		result, err := pc.ConvertJSONWithPrecision(jsonStr)
		assert.NoError(t, err)

		m, ok := result.(map[string]any)
		assert.True(t, ok)

		data := m["data"].(map[string]any)
		items := data["items"].([]any)

		item0 := items[0].(map[string]any)
		assert.Equal(t, "9007199254740992", item0["value"])

		item1 := items[1].(map[string]any)
		assert.Equal(t, int64(123), item1["value"])
	})

	t.Run("Should handle invalid JSON gracefully", func(t *testing.T) {
		pc := NewPrecisionConverter()

		// Test malformed JSON
		_, err := pc.ConvertJSONWithPrecision(`{"invalid": json}`)
		require.Error(t, err, "Invalid JSON should return an error")
		assert.Contains(t, err.Error(), "invalid", "Error should indicate JSON parsing failure")

		// Test empty JSON
		result, err := pc.ConvertJSONWithPrecision(`{}`)
		require.NoError(t, err, "Empty JSON should parse successfully")
		m, ok := result.(map[string]any)
		require.True(t, ok, "Empty JSON should return empty map")
		assert.Empty(t, m, "Empty JSON result should be empty map")
	})
}

func TestPrecisionConverter_EdgeCases(t *testing.T) {
	t.Run("Should handle boundary conditions and edge cases correctly", func(t *testing.T) {
		pc := NewPrecisionConverter()

		// Test zero values
		result := pc.ConvertWithPrecision("0")
		assert.Equal(t, int64(0), result, "Zero should be converted to int64")

		result = pc.ConvertWithPrecision("0.0")
		assert.Equal(t, int64(0), result, "Zero with decimal point should be converted to int64 as it's a whole number")

		// Test negative numbers
		result = pc.ConvertWithPrecision("-123")
		assert.Equal(t, int64(-123), result, "Negative integers should be converted to int64")

		result = pc.ConvertWithPrecision("-9007199254740992")
		assert.Equal(t, "-9007199254740992", result, "Large negative integers should be preserved as strings")

		result = pc.ConvertWithPrecision("-0.123456789123456789")
		assert.Equal(t, "-0.123456789123456789", result, "High precision negative decimals should be preserved")

		// Test leading/trailing zeros
		result = pc.ConvertWithPrecision("000123")
		assert.Equal(t, int64(123), result, "Leading zeros should be handled correctly")

		result = pc.ConvertWithPrecision("123.000")
		assert.Equal(
			t,
			int64(123),
			result,
			"Trailing zeros should be handled correctly and converted to int64 as it's a whole number",
		)

		// Test special float values as strings (edge case)
		result = pc.ConvertWithPrecision("NaN")
		assert.Equal(t, "NaN", result, "NaN as string should pass through unchanged")

		result = pc.ConvertWithPrecision("Infinity")
		assert.Equal(t, "Infinity", result, "Infinity as string should pass through unchanged")
	})
}
