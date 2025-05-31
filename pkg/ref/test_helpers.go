package ref

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name        string
	input       string
	options     []EvalConfigOption
	want        Node
	wantErr     bool
	errContains string
}

func runTestCases(t *testing.T, cases []testCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProcessBytes([]byte(tc.input), tc.options...)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// MustEval processes a YAML file and returns the result, failing the test on error.
func MustEval(t testing.TB, path string, options ...EvalConfigOption) Node {
	t.Helper()
	result, err := ProcessFile(path, options...)
	if err != nil {
		t.Fatalf("failed to evaluate %s: %v", path, err)
	}
	return result
}

// MustEvalBytes processes YAML bytes and returns the result, failing the test on error.
func MustEvalBytes(t testing.TB, data []byte, options ...EvalConfigOption) Node {
	t.Helper()
	result, err := ProcessBytes(data, options...)
	if err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}
	// Normalize numeric values to match JSON behavior
	return normalizeResult(result)
}

// TestDataPath returns the path to a test data file.
func TestDataPath(t testing.TB, filename string) string {
	t.Helper()
	return filepath.Join("testdata", filename)
}

func normalizeNumbers(v any) {
	switch vv := v.(type) {
	case map[string]any:
		for k, x := range vv {
			if i, ok := x.(int); ok {
				vv[k] = float64(i)
			} else {
				normalizeNumbers(x)
			}
		}
	case []any:
		for i, x := range vv {
			if j, ok := x.(int); ok {
				vv[i] = float64(j)
			} else {
				normalizeNumbers(x)
			}
		}
	}
}

// normalizeResult ensures all numeric values are float64 to match JSON unmarshalling behavior
func normalizeResult(v any) any {
	switch vv := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, x := range vv {
			result[k] = normalizeResult(x)
		}
		return result
	case []any:
		result := make([]any, len(vv))
		for i, x := range vv {
			result[i] = normalizeResult(x)
		}
		return result
	case int:
		return float64(vv)
	case int64:
		return float64(vv)
	case int32:
		return float64(vv)
	default:
		return vv
	}
}
