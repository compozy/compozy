package ref

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Merge Strategy Matrix Tests
// -----------------------------------------------------------------------------

func TestMergeStrategy_Matrix(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		ref     any
		inline  any
		want    any
		wantErr bool
	}{
		// Replace Mode Tests
		{
			name:   "replace mode with maps",
			mode:   ModeReplace,
			ref:    map[string]any{"a": 1, "b": 2},
			inline: map[string]any{"c": 3, "d": 4},
			want:   map[string]any{"a": 1, "b": 2},
		},
		{
			name:   "replace mode with different types",
			mode:   ModeReplace,
			ref:    "string_value",
			inline: map[string]any{"key": "value"},
			want:   "string_value",
		},
		{
			name:   "replace mode with nil ref",
			mode:   ModeReplace,
			ref:    nil,
			inline: map[string]any{"key": "value"},
			want:   nil,
		},

		// Append Mode Tests
		{
			name:   "append arrays",
			mode:   ModeAppend,
			ref:    []any{"a", "b"},
			inline: []any{"c", "d"},
			want:   []any{"c", "d", "a", "b"}, // inline first, then ref
		},
		{
			name:   "append primitive arrays",
			mode:   ModeAppend,
			ref:    []any{1, 2},
			inline: []any{3, 4},
			want:   []any{3, 4, 1, 2},
		},
		{
			name:   "append empty arrays",
			mode:   ModeAppend,
			ref:    []any{},
			inline: []any{"a"},
			want:   []any{"a"},
		},
		{
			name:    "append with nil ref",
			mode:    ModeAppend,
			ref:     nil,
			inline:  []any{"a"},
			wantErr: true, // nil is not considered an array
		},
		{
			name:    "append mode type mismatch - ref not array",
			mode:    ModeAppend,
			ref:     "not_array",
			inline:  []any{"a"},
			wantErr: true,
		},
		{
			name:    "append mode type mismatch - inline not array",
			mode:    ModeAppend,
			ref:     []any{"a"},
			inline:  "not_array",
			wantErr: true,
		},
		{
			name:    "append mode both not arrays",
			mode:    ModeAppend,
			ref:     map[string]any{"a": 1},
			inline:  map[string]any{"b": 2},
			wantErr: true,
		},

		// Merge Mode Tests - Maps
		{
			name:   "merge simple maps",
			mode:   ModeMerge,
			ref:    map[string]any{"a": 1, "b": 2},
			inline: map[string]any{"b": 3, "c": 4},
			want:   map[string]any{"a": 1, "b": 2, "c": 4}, // ref wins for 'b'
		},
		{
			name: "merge nested maps",
			mode: ModeMerge,
			ref: map[string]any{
				"nested": map[string]any{"x": 10, "y": 20},
				"top":    "ref_value",
			},
			inline: map[string]any{
				"nested": map[string]any{"y": 30, "z": 40},
				"other":  "inline_value",
			},
			want: map[string]any{
				"nested": map[string]any{"x": 10, "y": 20, "z": 40}, // ref wins for 'y'
				"top":    "ref_value",
				"other":  "inline_value",
			},
		},
		{
			name:   "merge with empty inline map",
			mode:   ModeMerge,
			ref:    map[string]any{"a": 1},
			inline: map[string]any{},
			want:   map[string]any{"a": 1},
		},
		{
			name:   "merge with empty ref map",
			mode:   ModeMerge,
			ref:    map[string]any{},
			inline: map[string]any{"a": 1},
			want:   map[string]any{"a": 1},
		},

		// Merge Mode Tests - Arrays
		{
			name:   "merge arrays (union)",
			mode:   ModeMerge,
			ref:    []any{"a", "b"},
			inline: []any{"c", "d"},
			want:   []any{"c", "d", "a", "b"}, // inline first, then ref
		},
		{
			name:   "merge mixed type arrays",
			mode:   ModeMerge,
			ref:    []any{1, "string", true},
			inline: []any{"other", 2},
			want:   []any{"other", 2, 1, "string", true},
		},

		// Merge Mode Tests - Different Types
		{
			name:   "merge different types - ref wins",
			mode:   ModeMerge,
			ref:    "string_ref",
			inline: 42,
			want:   "string_ref",
		},
		{
			name:   "merge map vs array - ref wins",
			mode:   ModeMerge,
			ref:    map[string]any{"key": "value"},
			inline: []any{"item1", "item2"},
			want:   map[string]any{"key": "value"},
		},
		{
			name:   "merge with nil ref",
			mode:   ModeMerge,
			ref:    nil,
			inline: map[string]any{"key": "value"},
			want:   map[string]any{"key": "value"},
		},
		{
			name:   "merge with nil inline",
			mode:   ModeMerge,
			ref:    map[string]any{"key": "value"},
			inline: nil,
			want:   map[string]any{"key": "value"},
		},
		{
			name:   "merge with both nil",
			mode:   ModeMerge,
			ref:    nil,
			inline: nil,
			want:   nil,
		},

		// Complex Nested Structures
		{
			name: "merge deeply nested structures",
			mode: ModeMerge,
			ref: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"arrays": []any{"ref1", "ref2"},
						"value":  "from_ref",
					},
				},
			},
			inline: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"arrays": []any{"inline1"},
						"other":  "from_inline",
					},
					"sibling": "inline_sibling",
				},
			},
			want: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"arrays": []any{"inline1", "ref1", "ref2"}, // Arrays merged
						"value":  "from_ref",                       // Ref wins
						"other":  "from_inline",                    // Only in inline
					},
					"sibling": "inline_sibling", // Only in inline
				},
			},
		},

		// Edge Cases with Flat Maps (Fast Path)
		{
			name:   "merge flat maps - fast path",
			mode:   ModeMerge,
			ref:    map[string]any{"str": "ref", "num": 42, "bool": true},
			inline: map[string]any{"str": "inline", "other": "value"},
			want:   map[string]any{"str": "ref", "num": 42, "bool": true, "other": "value"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strategy, err := GetMergeStrategy(tc.mode)
			require.NoError(t, err)

			got, err := strategy.Merge(tc.ref, tc.inline)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// GetMergeStrategy Tests
// -----------------------------------------------------------------------------

func TestGetMergeStrategy(t *testing.T) {
	tests := []struct {
		mode     Mode
		wantErr  bool
		wantType string
	}{
		{ModeMerge, false, "*ref.DeepMergeStrategy"},
		{ModeReplace, false, "*ref.ReplaceStrategy"},
		{ModeAppend, false, "*ref.AppendStrategy"},
		{Mode("invalid"), true, ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			strategy, err := GetMergeStrategy(tc.mode)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, strategy)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, strategy)
				// Check the type name matches expected strategy
				typeName := fmt.Sprintf("%T", strategy)
				assert.Equal(t, tc.wantType, typeName)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Individual Strategy Tests
// -----------------------------------------------------------------------------

func TestReplaceStrategy(t *testing.T) {
	strategy := &ReplaceStrategy{}

	t.Run("Should always return ref value", func(t *testing.T) {
		testCases := []struct {
			ref    any
			inline any
		}{
			{"ref", "inline"},
			{42, "string"},
			{map[string]any{"key": "ref"}, map[string]any{"key": "inline"}},
			{[]any{"ref"}, []any{"inline"}},
			{nil, "something"},
		}

		for _, tc := range testCases {
			result, err := strategy.Merge(tc.ref, tc.inline)
			require.NoError(t, err)
			assert.Equal(t, tc.ref, result)
		}
	})
}

func TestAppendStrategy(t *testing.T) {
	strategy := &AppendStrategy{}

	t.Run("Should append arrays correctly", func(t *testing.T) {
		result, err := strategy.Merge([]any{"a", "b"}, []any{"c", "d"})
		require.NoError(t, err)
		assert.Equal(t, []any{"c", "d", "a", "b"}, result)
	})

	t.Run("Should handle different slice types", func(t *testing.T) {
		// Test with different underlying slice types
		refSlice := []string{"a", "b"}
		inlineSlice := []string{"c", "d"}

		result, err := strategy.Merge(refSlice, inlineSlice)
		require.NoError(t, err)
		assert.Equal(t, []any{"c", "d", "a", "b"}, result)
	})

	t.Run("Should error on non-slice inputs", func(t *testing.T) {
		_, err := strategy.Merge("not_slice", []any{"a"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "append mode requires both values to be arrays")

		_, err = strategy.Merge([]any{"a"}, "not_slice")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "append mode requires both values to be arrays")
	})
}

func TestDeepMergeStrategy_FastPath(t *testing.T) {
	strategy := &DeepMergeStrategy{}

	t.Run("Should use fast path for flat maps", func(t *testing.T) {
		// Flat maps (no nested structures) should use the optimized fast path
		ref := map[string]any{
			"str":  "ref_value",
			"num":  42,
			"bool": true,
		}
		inline := map[string]any{
			"str":   "inline_value",
			"other": "additional",
		}

		result, err := strategy.Merge(ref, inline)
		require.NoError(t, err)

		expected := map[string]any{
			"str":   "ref_value", // ref wins
			"num":   42,
			"bool":  true,
			"other": "additional",
		}
		assert.Equal(t, expected, result)
	})

	t.Run("Should use slow path for nested maps", func(t *testing.T) {
		// Maps with nested structures should use recursive merging
		ref := map[string]any{
			"nested": map[string]any{"key": "ref"},
			"flat":   "ref_flat",
		}
		inline := map[string]any{
			"nested": map[string]any{"key": "inline", "other": "inline_other"},
			"flat":   "inline_flat",
		}

		result, err := strategy.Merge(ref, inline)
		require.NoError(t, err)

		expected := map[string]any{
			"nested": map[string]any{
				"key":   "ref",          // ref wins
				"other": "inline_other", // only in inline
			},
			"flat": "ref_flat", // ref wins
		}
		assert.Equal(t, expected, result)
	})
}
