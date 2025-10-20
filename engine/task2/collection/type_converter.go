package collection

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"

	"github.com/compozy/compozy/engine/task2/shared"
)

// TypeConverter handles conversion of various types to slice
type TypeConverter struct{}

// NewTypeConverter creates a new type converter
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// NewTypeConverterWithPrecision creates a new type converter
// Note: Precision handling is now done by the template engine
func NewTypeConverterWithPrecision() *TypeConverter {
	return &TypeConverter{}
}

// ConvertToSlice converts various types to a slice
func (tc *TypeConverter) ConvertToSlice(value any) []any {
	if value == nil {
		return []any{}
	}
	// Note: Precision conversion is now handled by the template engine
	// when WithPrecisionPreservation is enabled
	// Handle slice types
	if result := tc.convertSliceTypes(value); result != nil {
		return result
	}
	// Handle map types
	if result := tc.convertMapTypes(value); result != nil {
		return result
	}
	// Handle string types (including range expressions)
	if result := tc.convertStringTypes(value); result != nil {
		return result
	}
	// Handle primitive types
	return tc.convertPrimitiveTypes(value)
}

// convertSliceTypes handles conversion of slice types
func (tc *TypeConverter) convertSliceTypes(value any) []any {
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
	}
	return nil
}

// convertMapTypes handles conversion of map types in deterministic order
func (tc *TypeConverter) convertMapTypes(value any) []any {
	if v, ok := value.(map[string]any); ok {
		result := make([]any, 0, len(v))
		keys := shared.SortedMapKeys(v)
		for _, k := range keys {
			val := v[k]
			result = append(result, map[string]any{
				"key":   k,
				"value": val,
			})
		}
		return result
	}
	return nil
}

// convertStringTypes handles conversion of string types including range expressions
func (tc *TypeConverter) convertStringTypes(value any) []any {
	if v, ok := value.(string); ok {
		if rangeItems := tc.parseRangeExpression(v); rangeItems != nil {
			return rangeItems
		}
		return []any{v}
	}
	return nil
}

// convertPrimitiveTypes handles conversion of primitive types
func (tc *TypeConverter) convertPrimitiveTypes(value any) []any {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return []any{value}
	case float32, float64:
		return []any{value}
	case bool:
		return []any{value}
	case json.Number:
		// Convert json.Number to appropriate numeric type
		return []any{tc.handleJSONNumber(v)}
	default:
		return []any{value}
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
			// Check if float is within int range before converting
			if f > float64(math.MaxInt) || f < float64(math.MinInt) {
				return 0, fmt.Errorf("number out of int range: %s", s)
			}
			return int(f), nil
		}
		return 0, fmt.Errorf("not a whole number: %s", s)
	}
	// Try parsing as big int
	if bigInt, ok := new(big.Int).SetString(s, 10); ok {
		if bigInt.IsInt64() {
			i64 := bigInt.Int64()
			if i64 > math.MaxInt || i64 < math.MinInt {
				return 0, fmt.Errorf("number out of int range: %s", s)
			}
			return int(i64), nil
		}
		return 0, fmt.Errorf("number too large: %s", s)
	}
	return 0, fmt.Errorf("not a number: %s", s)
}

// parseCharacterRange parses Unicode character ranges like "a..z", "α..ω", or "你..我"
// Supports full Unicode character sets, not just ASCII letters
func (tc *TypeConverter) parseCharacterRange(start, end string) []any {
	startRunes := []rune(start)
	endRunes := []rune(end)
	if len(startRunes) != 1 || len(endRunes) != 1 {
		return nil
	}
	startChar := startRunes[0]
	endChar := endRunes[0]
	// Check if both are letters (using Unicode-aware function)
	if !unicode.IsLetter(startChar) || !unicode.IsLetter(endChar) {
		return nil
	}
	// Check if both are from the same Unicode script for consistency
	if !isSameScript(startChar, endChar) {
		return nil
	}
	// Handle reverse ranges
	if startChar > endChar {
		size := int(startChar-endChar) + 1
		// Prevent allocation overflow - limit range size to reasonable bounds
		if size > 10000 { // Max 10k characters in a range
			return nil
		}
		result := make([]any, size)
		for i := 0; i <= int(startChar-endChar); i++ {
			result[i] = string(startChar - rune(i))
		}
		return result
	}
	// Normal ascending range
	size := int(endChar-startChar) + 1
	// Prevent allocation overflow - limit range size to reasonable bounds
	if size > 10000 { // Max 10k characters in a range
		return nil
	}
	result := make([]any, size)
	for i := 0; i <= int(endChar-startChar); i++ {
		result[i] = string(startChar + rune(i))
	}
	return result
}

// isSameScript checks if two runes belong to the same Unicode script
// This ensures character ranges make sense (e.g., "a..z" or "α..ω" but not "a..α")
func isSameScript(r1, r2 rune) bool {
	// For ASCII letters, check if both are ASCII and same case
	if isASCIILetter(r1) && isASCIILetter(r2) {
		return unicode.IsUpper(r1) == unicode.IsUpper(r2)
	}
	// For non-ASCII, check common Unicode scripts
	scripts := []*unicode.RangeTable{
		unicode.Greek, unicode.Cyrillic, unicode.Arabic, unicode.Hebrew,
		unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai,
		unicode.Devanagari, unicode.Tamil, unicode.Telugu, unicode.Gujarati,
	}
	for _, script := range scripts {
		if unicode.Is(script, r1) && unicode.Is(script, r2) {
			return true
		}
	}
	// If neither are ASCII letters and don't match known scripts,
	// allow if they're close in Unicode codepoint space (same block)
	const blockSize = 128 // Approximate Unicode block size
	return (r1 / blockSize) == (r2 / blockSize)
}

// isASCIILetter checks if a rune is an ASCII letter (a-z, A-Z)
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// handleJSONNumber processes json.Number type to preserve precision
func (tc *TypeConverter) handleJSONNumber(num json.Number) any {
	numStr := string(num)
	// Try int64 first
	if i, err := strconv.ParseInt(numStr, 10, 64); err == nil {
		return i
	}
	// Try float64
	if f, err := strconv.ParseFloat(numStr, 64); err == nil {
		return f
	}
	// Cannot parse as number, return as string
	return numStr
}
