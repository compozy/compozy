package tplengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
}
