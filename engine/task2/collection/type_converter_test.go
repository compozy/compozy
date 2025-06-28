package collection

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeConverter_parseCharacterRange_UnicodeSupport(t *testing.T) {
	tc := NewTypeConverter()

	tests := []struct {
		name     string
		start    string
		end      string
		expected []any
	}{
		{
			name:     "Should handle ASCII lowercase range",
			start:    "a",
			end:      "d",
			expected: []any{"a", "b", "c", "d"},
		},
		{
			name:     "Should handle ASCII uppercase range",
			start:    "A",
			end:      "D",
			expected: []any{"A", "B", "C", "D"},
		},
		{
			name:     "Should handle Greek character range",
			start:    "α",
			end:      "δ",
			expected: []any{"α", "β", "γ", "δ"},
		},
		{
			name:     "Should handle Cyrillic character range",
			start:    "а",
			end:      "г",
			expected: []any{"а", "б", "в", "г"},
		},
		{
			name:     "Should handle reverse ASCII range",
			start:    "d",
			end:      "a",
			expected: []any{"d", "c", "b", "a"},
		},
		{
			name:     "Should handle reverse Greek range",
			start:    "δ",
			end:      "α",
			expected: []any{"δ", "γ", "β", "α"},
		},
		{
			name:     "Should handle single character range",
			start:    "x",
			end:      "x",
			expected: []any{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.parseCharacterRange(tt.start, tt.end)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConverter_parseCharacterRange_ErrorCases(t *testing.T) {
	tc := NewTypeConverter()

	tests := []struct {
		name  string
		start string
		end   string
	}{
		{
			name:  "Should reject mixed scripts (ASCII and Greek)",
			start: "a",
			end:   "α",
		},
		{
			name:  "Should reject mixed scripts (Greek and Cyrillic)",
			start: "α",
			end:   "а",
		},
		{
			name:  "Should reject mixed case ASCII",
			start: "a",
			end:   "Z",
		},
		{
			name:  "Should reject numbers",
			start: "1",
			end:   "5",
		},
		{
			name:  "Should reject multi-character start",
			start: "ab",
			end:   "z",
		},
		{
			name:  "Should reject multi-character end",
			start: "a",
			end:   "xyz",
		},
		{
			name:  "Should reject non-letters",
			start: "!",
			end:   "@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.parseCharacterRange(tt.start, tt.end)
			assert.Nil(t, result)
		})
	}
}

func TestTypeConverter_parseNumber_BigIntPrecision(t *testing.T) {
	tc := NewTypeConverter()

	tests := []struct {
		name        string
		input       string
		expected    int
		expectError bool
	}{
		{
			name:     "Should handle regular integer",
			input:    "42",
			expected: 42,
		},
		{
			name:     "Should handle negative integer",
			input:    "-42",
			expected: -42,
		},
		{
			name:     "Should handle large integer within int range",
			input:    "2147483647",
			expected: 2147483647,
		},
		{
			name:     "Should handle whole number as float",
			input:    "42.0",
			expected: 42,
		},
		{
			name:        "Should reject non-whole float",
			input:       "42.5",
			expectError: true,
		},
		{
			name:        "Should reject non-numeric string",
			input:       "abc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.parseNumber(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTypeConverter_parseNumber_BoundaryConditions(t *testing.T) {
	tc := NewTypeConverter()

	t.Run("Should handle maximum int value", func(t *testing.T) {
		maxIntStr := "9223372036854775807" // math.MaxInt64 on 64-bit systems
		result, err := tc.parseNumber(maxIntStr)
		require.NoError(t, err)
		assert.Equal(t, math.MaxInt64, result)
	})

	t.Run("Should handle minimum int value", func(t *testing.T) {
		minIntStr := "-9223372036854775808" // math.MinInt64 on 64-bit systems
		result, err := tc.parseNumber(minIntStr)
		require.NoError(t, err)
		assert.Equal(t, math.MinInt64, result)
	})

	t.Run("Should reject number too large for int64", func(t *testing.T) {
		// A valid big int that exceeds int64 range significantly
		tooLargeStr := "18446744073709551616" // Much larger than MaxInt64
		_, err := tc.parseNumber(tooLargeStr)
		assert.Error(t, err)
		assert.True(t, err != nil, "Expected an error for number too large")
	})

	t.Run("Should reject invalid big number string", func(t *testing.T) {
		// A string that can't be parsed as any number
		invalidStr := "not_a_number_at_all"
		_, err := tc.parseNumber(invalidStr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a number")
	})
}

func TestTypeConverter_isSameScript(t *testing.T) {
	tests := []struct {
		name     string
		r1       rune
		r2       rune
		expected bool
	}{
		{
			name:     "Should match ASCII lowercase letters",
			r1:       'a',
			r2:       'z',
			expected: true,
		},
		{
			name:     "Should match ASCII uppercase letters",
			r1:       'A',
			r2:       'Z',
			expected: true,
		},
		{
			name:     "Should not match mixed case ASCII",
			r1:       'a',
			r2:       'Z',
			expected: false,
		},
		{
			name:     "Should match Greek letters",
			r1:       'α',
			r2:       'ω',
			expected: true,
		},
		{
			name:     "Should match Cyrillic letters",
			r1:       'а',
			r2:       'я',
			expected: true,
		},
		{
			name:     "Should not match different scripts",
			r1:       'a',
			r2:       'α',
			expected: false,
		},
		{
			name:     "Should not match Greek and Cyrillic",
			r1:       'α',
			r2:       'а',
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSameScript(tt.r1, tt.r2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConverter_isASCIILetter(t *testing.T) {
	tests := []struct {
		name     string
		r        rune
		expected bool
	}{
		{
			name:     "Should identify ASCII lowercase letter",
			r:        'a',
			expected: true,
		},
		{
			name:     "Should identify ASCII uppercase letter",
			r:        'A',
			expected: true,
		},
		{
			name:     "Should reject number",
			r:        '5',
			expected: false,
		},
		{
			name:     "Should reject symbol",
			r:        '!',
			expected: false,
		},
		{
			name:     "Should reject Greek letter",
			r:        'α',
			expected: false,
		},
		{
			name:     "Should reject Cyrillic letter",
			r:        'а',
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isASCIILetter(tt.r)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConverter_parseRangeExpression_IntegrationTests(t *testing.T) {
	tc := NewTypeConverter()

	tests := []struct {
		name     string
		expr     string
		expected []any
	}{
		{
			name:     "Should parse numeric range",
			expr:     "1..3",
			expected: []any{1, 2, 3},
		},
		{
			name:     "Should parse character range",
			expr:     "a..c",
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "Should parse Unicode range",
			expr:     "α..γ",
			expected: []any{"α", "β", "γ"},
		},
		{
			name:     "Should handle whitespace around dots",
			expr:     " a .. c ",
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "Should return nil for invalid range",
			expr:     "invalid",
			expected: nil,
		},
		{
			name:     "Should return nil for mixed script range",
			expr:     "a..α",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.parseRangeExpression(tt.expr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConverter_ConvertToSlice_FullWorkflow(t *testing.T) {
	tc := NewTypeConverter()

	tests := []struct {
		name     string
		input    any
		expected []any
	}{
		{
			name:     "Should convert ASCII character range",
			input:    "a..c",
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "Should convert Unicode character range",
			input:    "α..γ",
			expected: []any{"α", "β", "γ"},
		},
		{
			name:     "Should convert numeric range",
			input:    "1..3",
			expected: []any{1, 2, 3},
		},
		{
			name:     "Should convert regular string",
			input:    "hello",
			expected: []any{"hello"},
		},
		{
			name:     "Should convert slice",
			input:    []string{"a", "b", "c"},
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "Should convert primitive",
			input:    42,
			expected: []any{42},
		},
		{
			name:     "Should handle nil",
			input:    nil,
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.ConvertToSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeConverter_PrecisionHandling(t *testing.T) {
	t.Run("Should handle strings without conversion", func(t *testing.T) {
		// Note: Precision handling is now done by the template engine
		// TypeConverter just treats strings as strings
		tc := NewTypeConverterWithPrecision()

		// Large integer remains as string
		largeInt := "9007199254740992" // MAX_SAFE_INTEGER + 1
		result := tc.ConvertToSlice(largeInt)

		require.Len(t, result, 1)
		assert.IsType(t, "", result[0])
		assert.Equal(t, "9007199254740992", result[0])
	})

	t.Run("Should handle JSON numbers with standard conversion", func(t *testing.T) {
		tc := NewTypeConverterWithPrecision()

		// JSON numbers are converted to int64 or float64 by handleJSONNumber
		jsonNum := json.Number("42")
		result := tc.ConvertToSlice(jsonNum)

		require.Len(t, result, 1)
		assert.IsType(t, int64(0), result[0])
		assert.Equal(t, int64(42), result[0])
	})

	t.Run("Should handle large JSON numbers", func(t *testing.T) {
		tc := NewTypeConverterWithPrecision()

		// Large JSON number that fits in int64
		jsonNum := json.Number("9007199254740992")
		result := tc.ConvertToSlice(jsonNum)

		require.Len(t, result, 1)
		assert.IsType(t, int64(0), result[0])
		assert.Equal(t, int64(9007199254740992), result[0])
	})

	t.Run("Should handle JSON floats", func(t *testing.T) {
		tc := NewTypeConverterWithPrecision()

		jsonNum := json.Number("3.14159")
		result := tc.ConvertToSlice(jsonNum)

		require.Len(t, result, 1)
		assert.IsType(t, float64(0), result[0])
		assert.Equal(t, 3.14159, result[0])
	})

	t.Run("Should handle very large JSON numbers as float64", func(t *testing.T) {
		tc := NewTypeConverterWithPrecision()

		// Too large for int64, will be parsed as float64
		jsonNum := json.Number("99999999999999999999999999999999")
		result := tc.ConvertToSlice(jsonNum)

		require.Len(t, result, 1)
		assert.IsType(t, float64(0), result[0])
		// The value will be represented as 1e+32 in float64
		assert.Equal(t, float64(1e+32), result[0])
	})

	t.Run("Should handle unparseable JSON numbers as strings", func(t *testing.T) {
		tc := NewTypeConverterWithPrecision()

		// Invalid number format
		jsonNum := json.Number("not-a-number")
		result := tc.ConvertToSlice(jsonNum)

		require.Len(t, result, 1)
		assert.IsType(t, "", result[0])
		assert.Equal(t, "not-a-number", result[0])
	})
}
