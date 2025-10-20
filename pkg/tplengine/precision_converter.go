package tplengine

import (
	"encoding/json"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	// JavaScriptMaxSafeInteger represents the maximum safe integer in JavaScript (2^53 - 1)
	JavaScriptMaxSafeInteger = 9007199254740991
	// JavaScriptMinSafeInteger represents the minimum safe integer in JavaScript -(2^53 - 1)
	JavaScriptMinSafeInteger = -9007199254740991
	// Float64SignificantDigits represents the maximum number of significant digits
	// that can be accurately represented in a float64
	Float64SignificantDigits = 15
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
	dec, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	if dec.IsInteger() {
		bigInt := dec.BigInt()

		if bigInt.IsInt64() {
			i64 := bigInt.Int64()
			if i64 >= JavaScriptMinSafeInteger && i64 <= JavaScriptMaxSafeInteger {
				return i64
			}
		}
		return s
	}
	f64, _ := dec.Float64()
	backDec := decimal.NewFromFloat(f64)
	if !dec.Equal(backDec) {
		return s
	}
	if countSignificantDigits(s) > Float64SignificantDigits {
		return s
	}
	return f64
}

// countSignificantDigits counts the number of significant digits in a numeric string
func countSignificantDigits(s string) int {
	s = strings.TrimSpace(s)
	if strings.Contains(strings.ToLower(s), "e") {
		parts := strings.Split(strings.ToLower(s), "e")
		if len(parts) > 0 {
			s = parts[0]
		}
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" || s == "." {
		return 1
	}
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
