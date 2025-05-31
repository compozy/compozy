package ref

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// ApplyMergeMode Matrix Tests
// -----------------------------------------------------------------------------

func TestApplyMergeMode_Matrix(t *testing.T) {
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
			mode:   ModeAppend,
			ref:    nil,
			inline: []any{"a"},
			want:   []any{"a"}, // if ref is nil, inline slice is returned
		},
		{
			name: "append mode with nil inline",
			mode: ModeAppend,
			ref:  []any{"a"},
			inline: nil,
			want: []any{"a"}, // if inline is nil, ref slice is returned
		},
		{
			name: "append mode with both nil",
			mode: ModeAppend,
			ref:  nil,
			inline: nil,
			want: []any{}, // empty slice if both nil
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
			ref:    []any{"a", "b"}, // ref is a slice
			inline: []any{"c", "d"}, // inline is a slice
			// According to new mergeValues: if not both maps, refValue takes precedence.
			want: []any{"a", "b"},
		},
		{
			name:   "merge mixed type arrays - ref wins as not maps",
			mode:   ModeMerge,
			ref:    []any{1, "string", true},
			inline: []any{"other", 2},
			// According to new mergeValues: if not both maps, refValue takes precedence.
			want: []any{1, "string", true},
		},

		// Merge Mode Tests - Different Types (refValue wins if not both maps)
		{
			name:   "merge different types - ref wins (string vs int)",
			mode:   ModeMerge,
			ref:    "string_ref",
			inline: 42,
			want:   "string_ref",
		},
		{
			name:   "merge map vs array - ref wins (map vs slice)",
			mode:   ModeMerge,
			ref:    map[string]any{"key": "value"},
			inline: []any{"item1", "item2"},
			want:   map[string]any{"key": "value"},
		},
		{
			name:   "merge array vs map - ref wins (slice vs map)",
			mode:   ModeMerge,
			ref:    []any{"item1", "item2"},
			inline: map[string]any{"key": "value"},
			want:   []any{"item1", "item2"},
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
			// Create a Ref instance with the specified mode
			refInstance := &Ref{Mode: tc.mode}

			got, err := refInstance.ApplyMergeMode(tc.ref, tc.inline)

			if tc.wantErr {
				assert.Error(t, err, fmt.Sprintf("Expected an error for mode %s", tc.mode))
			} else {
				require.NoError(t, err, fmt.Sprintf("Did not expect an error for mode %s", tc.mode))
				assert.Equal(t, tc.want, got, fmt.Sprintf("Output mismatch for mode %s", tc.mode))
			}
		})
	}
}

// Note: TestGetMergeStrategy, TestReplaceStrategy, TestAppendStrategy,
// and TestDeepMergeStrategy_FastPath have been removed as the underlying
// strategies and GetMergeStrategy function are no longer used.
// The matrix test TestApplyMergeMode_Matrix now covers these behaviors
// through the ApplyMergeMode method.
