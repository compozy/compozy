package runtime_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolIDValidation tests the validateToolID function through public APIs
func TestToolIDValidation(t *testing.T) {
	tests := []struct {
		name          string
		toolID        string
		shouldSucceed bool
		expectedError string
	}{
		{
			name:          "Should accept valid tool ID",
			toolID:        "simple-tool",
			shouldSucceed: true,
		},
		{
			name:          "Should accept tool ID with path",
			toolID:        "category/tool-name",
			shouldSucceed: true,
		},
		{
			name:          "Should accept tool ID with dots",
			toolID:        "tool.name",
			shouldSucceed: true,
		},
		{
			name:          "Should reject empty tool ID",
			toolID:        "",
			shouldSucceed: false,
			expectedError: "tool_id cannot be empty",
		},
		{
			name:          "Should reject tool ID with path traversal",
			toolID:        "../malicious",
			shouldSucceed: false,
			expectedError: "directory traversal patterns",
		},
		{
			name:          "Should reject tool ID with directory traversal",
			toolID:        "tool/../../../etc/passwd",
			shouldSucceed: false,
			expectedError: "path traversal or invalid path components",
		},
		{
			name:          "Should reject absolute path",
			toolID:        "/etc/passwd",
			shouldSucceed: false,
			expectedError: "absolute path",
		},
		{
			name:          "Should reject tool ID starting with dot",
			toolID:        ".hidden-tool",
			shouldSucceed: false,
			expectedError: "start with dot",
		},
		{
			name:          "Should reject tool ID with multiple dots",
			toolID:        "tool...name",
			shouldSucceed: false,
			expectedError: "directory traversal patterns",
		},
		{
			name:          "Should reject tool ID with invalid characters",
			toolID:        "tool@name",
			shouldSucceed: false,
			expectedError: "invalid characters",
		},
		{
			name:          "Should reject tool ID with null bytes",
			toolID:        "tool\x00name",
			shouldSucceed: false,
			expectedError: "invalid characters",
		},
		{
			name:          "Should reject Unicode homoglyphs (normalized)",
			toolID:        "tοοl", // Contains Greek omicron instead of Latin o
			shouldSucceed: false,
			expectedError: "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test via ExecuteTool to exercise validateToolID
			tmpDir := t.TempDir()
			ctx := context.Background()

			if !runtime.IsBunAvailable() {
				t.Skip("Bun not available")
			}

			bm, err := runtime.NewBunManager(ctx, tmpDir, nil)
			require.NoError(t, err)

			toolExecID := core.MustNewID()
			input := &core.Input{}
			env := core.EnvMap{}

			_, err = bm.ExecuteTool(ctx, tt.toolID, toolExecID, input, nil, env)

			if tt.shouldSucceed {
				// Tool execution may fail for other reasons, but validation should pass
				// We only care that the error is NOT a validation error
				if err != nil {
					assert.NotContains(t, err.Error(), "invalid")
					assert.NotContains(t, err.Error(), "traversal")
					assert.NotContains(t, err.Error(), "absolute")
				}
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

// TestEnvironmentVariableValidation tests environment variable validation
func TestEnvironmentVariableValidation(t *testing.T) {
	tests := []struct {
		name          string
		env           core.EnvMap
		shouldSucceed bool
		expectedError string
	}{
		{
			name: "Should accept valid environment variables",
			env: core.EnvMap{
				"VALID_VAR":   "value",
				"ANOTHER_VAR": "another_value",
				"NUMBER_123":  "123",
			},
			shouldSucceed: true,
		},
		{
			name: "Should reject dangerous LD_PRELOAD variable",
			env: core.EnvMap{
				"LD_PRELOAD": "/malicious/lib.so",
			},
			shouldSucceed: false,
			expectedError: "not allowed for security reasons",
		},
		{
			name: "Should reject dangerous NODE_OPTIONS variable",
			env: core.EnvMap{
				"NODE_OPTIONS": "--require /malicious/script.js",
			},
			shouldSucceed: false,
			expectedError: "not allowed for security reasons",
		},
		{
			name: "Should reject dangerous DYLD_INSERT_LIBRARIES variable",
			env: core.EnvMap{
				"DYLD_INSERT_LIBRARIES": "/malicious/lib.dylib",
			},
			shouldSucceed: false,
			expectedError: "not allowed for security reasons",
		},
		{
			name: "Should reject environment variable with lowercase name",
			env: core.EnvMap{
				"lowercase_var": "value",
			},
			shouldSucceed: false,
			expectedError: "must contain only uppercase",
		},
		{
			name: "Should reject environment variable with invalid characters",
			env: core.EnvMap{
				"INVALID@VAR": "value",
			},
			shouldSucceed: false,
			expectedError: "must contain only uppercase",
		},
		{
			name: "Should reject environment variable with newline in value",
			env: core.EnvMap{
				"VALID_VAR": "value\nwith\nnewlines",
			},
			shouldSucceed: false,
			expectedError: "invalid characters (newline or null byte)",
		},
		{
			name: "Should reject environment variable with null byte in value",
			env: core.EnvMap{
				"VALID_VAR": "value\x00with\x00nulls",
			},
			shouldSucceed: false,
			expectedError: "invalid characters (newline or null byte)",
		},
		{
			name: "Should reject environment variable with carriage return",
			env: core.EnvMap{
				"VALID_VAR": "value\rwith\rcarriage\rreturns",
			},
			shouldSucceed: false,
			expectedError: "invalid characters (newline or null byte)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test via ExecuteTool to exercise validateAndAddEnvironmentVars
			tmpDir := t.TempDir()
			ctx := context.Background()

			if !runtime.IsBunAvailable() {
				t.Skip("Bun not available")
			}

			bm, err := runtime.NewBunManager(ctx, tmpDir, nil)
			require.NoError(t, err)

			toolExecID := core.MustNewID()
			input := &core.Input{}

			_, err = bm.ExecuteTool(ctx, "valid-tool", toolExecID, input, nil, tt.env)

			if tt.shouldSucceed {
				// Tool execution may fail for other reasons, but env validation should pass
				if err != nil {
					assert.NotContains(t, err.Error(), "environment variable")
					assert.NotContains(t, err.Error(), "security reasons")
				}
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}
