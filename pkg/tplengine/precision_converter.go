package tplengine

import (
	"encoding/json"
	"strings"

	"github.com/shopspring/decimal"
)

// PrecisionConverter handles numeric conversion with precision preservation
type PrecisionConverter struct{}

// NewPrecisionConverter creates a new precision converter
func NewPrecisionConverter() *PrecisionConverter {
	return &PrecisionConverter{}
}

// ConvertWithPrecision converts a value to its appropriate type while preserving numeric precision
func (pc *PrecisionConverter) ConvertWithPrecision(value any) any {
	switch v := value.(type) {
	case string:
		return pc.convertStringWithPrecision(v)
	case json.Number:
		return pc.convertStringWithPrecision(string(v))
	default:
		return v
	}
}

// convertStringWithPrecision converts string values to appropriate numeric types
// while preserving precision using shopspring/decimal for validation
func (pc *PrecisionConverter) convertStringWithPrecision(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Try parsing with decimal first to validate and check precision
	dec, err := decimal.NewFromString(s)
	if err != nil {
		// Not a valid number, return as string
		return s
	}

	// Check if it's a whole number
	if dec.IsInteger() {
		bigInt := dec.BigInt()

		// Check if it fits in int64 AND is within JavaScript's MAX_SAFE_INTEGER
		// JavaScript's MAX_SAFE_INTEGER is 2^53 - 1 = 9007199254740991
		if bigInt.IsInt64() {
			i64 := bigInt.Int64()
			if i64 >= -9007199254740991 && i64 <= 9007199254740991 {
				return i64
			}
		}
		// Too large for safe integer representation, preserve as string
		return s
	}

	// For decimal numbers, check if conversion to float64 loses precision
	f64, _ := dec.Float64()

	// Convert back to decimal to check if we lost precision
	backDec := decimal.NewFromFloat(f64)

	// Compare the original decimal with the back-converted one
	if !dec.Equal(backDec) {
		// Precision would be lost, keep as string
		return s
	}

	// Check if the decimal has too many significant digits for float64
	// Float64 can accurately represent about 15-17 significant digits
	if countSignificantDigits(s) > 15 {
		// Too many digits for accurate float64 representation
		return s
	}

	return f64
}

// countSignificantDigits counts the number of significant digits in a numeric string
func countSignificantDigits(s string) int {
	s = strings.TrimSpace(s)

	// Handle scientific notation
	if strings.Contains(strings.ToLower(s), "e") {
		// For scientific notation, count digits in the mantissa
		parts := strings.Split(strings.ToLower(s), "e")
		if len(parts) > 0 {
			s = parts[0]
		}
	}

	// Remove leading sign
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		s = s[1:]
	}

	// Remove leading zeros
	s = strings.TrimLeft(s, "0")

	// If empty after trimming zeros, it was "0" or "0.0" etc
	if s == "" || s == "." {
		return 1
	}

	// Count digits, excluding decimal point
	count := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			count++
		}
	}

	return count
}

// ConvertJSONWithPrecision parses JSON with numeric precision preservation
func (pc *PrecisionConverter) ConvertJSONWithPrecision(jsonStr string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	decoder.UseNumber() // Use json.Number to preserve precision

	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}

	// Convert json.Number values to appropriate types
	return pc.convertJSONValue(result), nil
}

// convertJSONValue recursively converts JSON values, handling json.Number types
func (pc *PrecisionConverter) convertJSONValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		return pc.ConvertWithPrecision(v)
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = pc.convertJSONValue(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = pc.convertJSONValue(val)
		}
		return result
	default:
		return v
	}
}
