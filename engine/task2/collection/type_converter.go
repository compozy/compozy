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
	if result := tc.convertSliceTypes(value); result != nil {
		return result
	}
	if result := tc.convertMapTypes(value); result != nil {
		return result
	}
	if result := tc.convertStringTypes(value); result != nil {
		return result
	}
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
		return []any{tc.handleJSONNumber(v)}
	default:
		return []any{value}
	}
}

// parseRangeExpression parses range expressions like "1..10" or "a..z"
func (tc *TypeConverter) parseRangeExpression(expr string) []any {
	expr = strings.TrimSpace(expr)
	if !strings.Contains(expr, "..") {
		return nil
	}
	parts := strings.Split(expr, "..")
	if len(parts) != 2 {
		return nil
	}
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	if numRange := tc.parseNumericRange(start, end); numRange != nil {
		return numRange
	}
	if charRange := tc.parseCharacterRange(start, end); charRange != nil {
		return charRange
	}
	return nil
}

// parseNumericRange parses numeric ranges like "1..10" or "10..1"
func (tc *TypeConverter) parseNumericRange(start, end string) []any {
	startNum, err1 := tc.parseNumber(start)
	endNum, err2 := tc.parseNumber(end)
	if err1 != nil || err2 != nil {
		return nil
	}
	if startNum > endNum {
		diff := startNum - endNum
		result := make([]any, diff+1)
		for i := 0; i <= diff; i++ {
			result[i] = startNum - i
		}
		return result
	}
	diff := endNum - startNum
	result := make([]any, diff+1)
	for i := 0; i <= diff; i++ {
		result[i] = startNum + i
	}
	return result
}

// parseNumber attempts to parse a string as an integer
func (tc *TypeConverter) parseNumber(s string) (int, error) {
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if f == math.Floor(f) {
			if f > float64(math.MaxInt) || f < float64(math.MinInt) {
				return 0, fmt.Errorf("number out of int range: %s", s)
			}
			return int(f), nil
		}
		return 0, fmt.Errorf("not a whole number: %s", s)
	}
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
	if !unicode.IsLetter(startChar) || !unicode.IsLetter(endChar) {
		return nil
	}
	if !isSameScript(startChar, endChar) {
		return nil
	}
	if startChar > endChar {
		size := int(startChar-endChar) + 1
		if size > 10000 { // Max 10k characters in a range
			return nil
		}
		result := make([]any, size)
		for i := 0; i <= int(startChar-endChar); i++ {
			result[i] = string(startChar - rune(i))
		}
		return result
	}
	size := int(endChar-startChar) + 1
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
	if isASCIILetter(r1) && isASCIILetter(r2) {
		return unicode.IsUpper(r1) == unicode.IsUpper(r2)
	}
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
	if i, err := strconv.ParseInt(numStr, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(numStr, 64); err == nil {
		return f
	}
	return numStr
}
