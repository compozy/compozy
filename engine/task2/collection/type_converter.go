package collection

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// TypeConverter handles conversion of various types to slice
type TypeConverter struct{}

// NewTypeConverter creates a new type converter
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// ConvertToSlice converts various types to a slice
func (tc *TypeConverter) ConvertToSlice(value any) []any {
	if value == nil {
		return []any{}
	}
	switch v := value.(type) {
	case []any:
		return v
	case []string:
		result := make([]any, len(v))
		for i, s := range v {
			result[i] = s
		}
		return result
	case []int:
		result := make([]any, len(v))
		for i, n := range v {
			result[i] = n
		}
		return result
	case []float64:
		result := make([]any, len(v))
		for i, f := range v {
			result[i] = f
		}
		return result
	case map[string]any:
		// For maps, create items with key and value
		result := make([]any, 0, len(v))
		for k, val := range v {
			result = append(result, map[string]any{
				"key":   k,
				"value": val,
			})
		}
		return result
	case string:
		// Try to parse as a range expression
		if rangeItems := tc.parseRangeExpression(v); rangeItems != nil {
			return rangeItems
		}
		// Single string becomes single item slice
		return []any{v}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// Single number becomes single item slice
		return []any{v}
	case float32, float64:
		// Single float becomes single item slice
		return []any{v}
	case bool:
		// Single bool becomes single item slice
		return []any{v}
	default:
		// For any other type, wrap it in a slice
		return []any{v}
	}
}

// parseRangeExpression parses range expressions like "1..10" or "a..z"
func (tc *TypeConverter) parseRangeExpression(expr string) []any {
	expr = strings.TrimSpace(expr)
	// Check for ".." in the expression
	if !strings.Contains(expr, "..") {
		return nil
	}
	parts := strings.Split(expr, "..")
	if len(parts) != 2 {
		return nil
	}
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	// Try numeric range first
	if numRange := tc.parseNumericRange(start, end); numRange != nil {
		return numRange
	}
	// Try character range
	if charRange := tc.parseCharacterRange(start, end); charRange != nil {
		return charRange
	}
	// Not a valid range expression
	return nil
}

// parseNumericRange parses numeric ranges like "1..10" or "10..1"
func (tc *TypeConverter) parseNumericRange(start, end string) []any {
	startNum, err1 := tc.parseNumber(start)
	endNum, err2 := tc.parseNumber(end)
	if err1 != nil || err2 != nil {
		return nil
	}
	// Handle reverse ranges
	if startNum > endNum {
		diff := startNum - endNum
		result := make([]any, diff+1)
		for i := 0; i <= diff; i++ {
			result[i] = startNum - i
		}
		return result
	}
	// Normal ascending range
	diff := endNum - startNum
	result := make([]any, diff+1)
	for i := 0; i <= diff; i++ {
		result[i] = startNum + i
	}
	return result
}

// parseNumber attempts to parse a string as an integer
func (tc *TypeConverter) parseNumber(s string) (int, error) {
	// Try parsing as integer
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	// Try parsing as float and convert to int
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		// Check if it's a whole number
		if f == math.Floor(f) {
			return int(f), nil
		}
		return 0, fmt.Errorf("not a whole number: %s", s)
	}
	// Try parsing as big int
	if bigInt, ok := new(big.Int).SetString(s, 10); ok {
		if bigInt.IsInt64() {
			return int(bigInt.Int64()), nil
		}
		return 0, fmt.Errorf("number too large: %s", s)
	}
	return 0, fmt.Errorf("not a number: %s", s)
}

// parseCharacterRange parses character ranges like "a..z" or "Z..A"
func (tc *TypeConverter) parseCharacterRange(start, end string) []any {
	if len(start) != 1 || len(end) != 1 {
		return nil
	}
	startChar := rune(start[0])
	endChar := rune(end[0])
	// Check if both are letters
	if !isLetter(startChar) || !isLetter(endChar) {
		return nil
	}
	// Check if both are same case
	if isUpperCase(startChar) != isUpperCase(endChar) {
		return nil
	}
	// Handle reverse ranges
	if startChar > endChar {
		result := make([]any, int(startChar-endChar)+1)
		for i := 0; i <= int(startChar-endChar); i++ {
			result[i] = string(startChar - rune(i))
		}
		return result
	}
	// Normal ascending range
	result := make([]any, int(endChar-startChar)+1)
	for i := 0; i <= int(endChar-startChar); i++ {
		result[i] = string(startChar + rune(i))
	}
	return result
}

// isLetter checks if a rune is a letter
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isUpperCase checks if a rune is uppercase
func isUpperCase(r rune) bool {
	return r >= 'A' && r <= 'Z'
}
