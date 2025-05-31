package ref

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Path Cache Correctness Tests
// -----------------------------------------------------------------------------

func TestPathCacheCorrectness(t *testing.T) {
	t.Run("Should not leak results between different paths", func(t *testing.T) {
		doc := map[string]any{
			"users": []any{
				map[string]any{
					"id":   1,
					"name": "Alice",
					"role": "admin",
				},
				map[string]any{
					"id":   2,
					"name": "Bob",
					"role": "user",
				},
			},
			"config": map[string]any{
				"timeout": 30,
				"retries": 3,
			},
			"schemas": []any{
				map[string]any{
					"id":   "user_schema",
					"type": "object",
				},
				map[string]any{
					"id":   "config_schema",
					"type": "object",
				},
			},
		}

		// Test multiple different paths on the same document
		testCases := []struct {
			path     string
			expected any
		}{
			{"users.0.name", "Alice"},
			{"users.1.name", "Bob"},
			{"users.#(role==\"admin\").name", "Alice"},
			{"users.#(role==\"user\").name", "Bob"},
			{"config.timeout", float64(30)},
			{"config.retries", float64(3)},
			{"schemas.0.id", "user_schema"},
			{"schemas.#(id==\"config_schema\").type", "object"},
		}

		metadata := &DocMetadata{}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				result, err := walkGJSONPath(doc, tc.path, metadata)
				require.NoError(t, err, "Path: %s", tc.path)
				assert.Equal(t, tc.expected, result, "Path: %s", tc.path)
			})
		}
	})

	t.Run("Should handle JSON caching consistently", func(t *testing.T) {
		doc := map[string]any{
			"data": map[string]any{
				"numbers": []any{1, 2, 3, 4, 5},
				"strings": []any{"a", "b", "c"},
				"nested": map[string]any{
					"deep": map[string]any{
						"value": "found",
					},
				},
			},
		}

		// Pre-marshal JSON for caching
		jsonBytes, err := json.Marshal(doc)
		require.NoError(t, err)

		metadata := &DocMetadata{
			CurrentDocJSON: jsonBytes,
		}

		// Test paths with and without cached JSON
		testPaths := []string{
			"data.numbers.2",
			"data.strings.1",
			"data.nested.deep.value",
			"data.numbers.#(==3)",
		}

		// First run with cached JSON
		cachedResults := make([]any, len(testPaths))
		for i, path := range testPaths {
			result, err := walkGJSONPath(doc, path, metadata)
			require.NoError(t, err)
			cachedResults[i] = result
		}

		// Second run without cached JSON (fresh metadata)
		freshMetadata := &DocMetadata{}
		for i, path := range testPaths {
			result, err := walkGJSONPath(doc, path, freshMetadata)
			require.NoError(t, err)
			assert.Equal(t, cachedResults[i], result, "Path: %s should return same result with and without cache", path)
		}
	})
}

// -----------------------------------------------------------------------------
// Integer Normalization Tests
// -----------------------------------------------------------------------------

func TestIntegerNormalization(t *testing.T) {
	t.Run("Should preserve integers when not forced to normalize", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    any
			expected any
		}{
			{"int", int(42), int(42)},
			{"int32", int32(42), int32(42)},
			{"int64", int64(42), int64(42)},
			{"float64", float64(42.5), float64(42.5)},
			{"string", "test", "test"},
			{"bool", true, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := normalizeValue(tc.input)
				assert.Equal(t, tc.expected, result)
				assert.Equal(t, tc.expected, result, "Type should be preserved: expected %T, got %T", tc.expected, result)
			})
		}
	})

	t.Run("Should convert integers to float64 when forced", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    any
			expected any
		}{
			{"int", int(42), float64(42)},
			{"int32", int32(42), float64(42)},
			{"int64", int64(42), float64(42)},
			{"float64", float64(42.5), float64(42.5)}, // Already float64
			{"string", "test", "test"},                // Not a number
			{"bool", true, true},                      // Not a number
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := normalizeValueRecursive(tc.input, true)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Should handle nested structures with integers", func(t *testing.T) {
		input := map[string]any{
			"number":   int(42),
			"array":    []any{int(1), int(2), int(3)},
			"nested":   map[string]any{"count": int(10)},
			"mixed":    []any{"text", int(123), float64(4.5)},
			"string":   "not a number",
			"existing": float64(7.5),
		}

		// Without forced normalization - should preserve types
		result := normalizeValue(input)
		resultMap, ok := result.(map[string]any)
		require.True(t, ok)

		assert.Equal(t, int(42), resultMap["number"])
		assert.Equal(t, "not a number", resultMap["string"])
		assert.Equal(t, float64(7.5), resultMap["existing"])

		// With forced normalization - should convert integers
		forced := normalizeValueRecursive(input, true)
		forcedMap, ok := forced.(map[string]any)
		require.True(t, ok)

		assert.Equal(t, float64(42), forcedMap["number"])
		assert.Equal(t, "not a number", forcedMap["string"])
		assert.Equal(t, float64(7.5), forcedMap["existing"])

		// Check nested array
		arrayResult, ok := forcedMap["array"].([]any)
		require.True(t, ok)
		assert.Equal(t, float64(1), arrayResult[0])
		assert.Equal(t, float64(2), arrayResult[1])
		assert.Equal(t, float64(3), arrayResult[2])

		// Check nested map
		nestedResult, ok := forcedMap["nested"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(10), nestedResult["count"])
	})

	t.Run("Should optimize performance by scanning first", func(t *testing.T) {
		// Test the hasIntegers optimization function
		testCases := []struct {
			name    string
			input   any
			hasInts bool
		}{
			{"primitive int", int(42), true},
			{"primitive string", "test", false},
			{"map with int", map[string]any{"num": int(5)}, true},
			{"map without int", map[string]any{"str": "test"}, false},
			{"array with int", []any{int(1), "test"}, true},
			{"array without int", []any{"a", "b"}, false},
			{"nested with int", map[string]any{"nested": map[string]any{"num": int(1)}}, true},
			{"deeply nested without int", map[string]any{"a": map[string]any{"b": "text"}}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := hasIntegers(tc.input)
				assert.Equal(t, tc.hasInts, result)
			})
		}
	})
}

// -----------------------------------------------------------------------------
// GJSON Path Edge Cases
// -----------------------------------------------------------------------------

func TestGJSONPathEdgeCases(t *testing.T) {
	t.Run("Should handle empty and root paths", func(t *testing.T) {
		doc := map[string]any{
			"root": "value",
		}

		// Empty path should return the whole document
		result, err := walkGJSONPath(doc, "", nil)
		require.NoError(t, err)
		assert.Equal(t, doc, result)

		// Valid path should work
		result, err = walkGJSONPath(doc, "root", nil)
		require.NoError(t, err)
		assert.Equal(t, "value", result)
	})

	t.Run("Should handle complex array filters", func(t *testing.T) {
		doc := map[string]any{
			"items": []any{
				map[string]any{"type": "user", "active": true, "count": 5},
				map[string]any{"type": "admin", "active": false, "count": 2},
				map[string]any{"type": "user", "active": true, "count": 8},
			},
		}

		testCases := []struct {
			path     string
			expected any
		}{
			{"items.#(type==\"user\").count", float64(5)},  // First match
			{"items.#(active==true).type", "user"},         // Boolean filter
			{"items.#(count>6).type", "user"},              // Numeric comparison
			{"items.#(type==\"admin\").count", float64(2)}, // Simple admin filter
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				result, err := walkGJSONPath(doc, tc.path, nil)
				require.NoError(t, err, "Path: %s", tc.path)
				assert.Equal(t, tc.expected, result, "Path: %s", tc.path)
			})
		}
	})

	t.Run("Should handle invalid paths gracefully", func(t *testing.T) {
		doc := map[string]any{
			"valid": "data",
		}

		invalidPaths := []string{
			"nonexistent",
			"valid.missing",
			"valid.0", // Not an array
			"items.#(id==\"missing\")",
		}

		for _, path := range invalidPaths {
			t.Run(path, func(t *testing.T) {
				_, err := walkGJSONPath(doc, path, nil)
				assert.Error(t, err, "Path '%s' should return an error", path)
				assert.Contains(t, err.Error(), "not found")
			})
		}
	})
}
