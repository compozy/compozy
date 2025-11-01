package tool

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	engineschema "github.com/compozy/compozy/engine/schema"
	nativeuser "github.com/compozy/compozy/engine/tool/nativeuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MinimalConfig(t *testing.T) {
	t.Run("Should create tool with minimal configuration", func(t *testing.T) {
		cfg, err := New(
			t.Context(),
			"test-tool",
			WithName("Test Tool"),
			WithDescription("A test tool"),
			WithRuntime("bun"),
			WithCode("export default () => {}"),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-tool", cfg.ID)
		assert.Equal(t, "Test Tool", cfg.Name)
		assert.Equal(t, "A test tool", cfg.Description)
		assert.Equal(t, "bun", cfg.Runtime)
		assert.Equal(t, "export default () => {}", cfg.Code)
		assert.Equal(t, string(core.ConfigTool), cfg.Resource)
	})
}

func TestNew_NativeHandler(t *testing.T) {
	nativeuser.Reset()
	t.Run("Should register native handler and normalize config", func(t *testing.T) {
		handler := func(_ context.Context, _ map[string]any, _ map[string]any) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		}
		cfg, err := New(
			t.Context(),
			"native-tool",
			WithName("Native Tool"),
			WithDescription("Executes in-process"),
			WithNativeHandler(handler),
		)
		require.NoError(t, err)
		assert.Equal(t, "native", cfg.Implementation)
		assert.Equal(t, "go", cfg.Runtime)
		definition, ok := nativeuser.Lookup("native-tool")
		require.True(t, ok)
		out, callErr := definition.Handler(t.Context(), map[string]any{}, map[string]any{})
		require.NoError(t, callErr)
		assert.Equal(t, map[string]any{"ok": true}, out)
	})

	t.Run("Should require native handler when runtime is go", func(t *testing.T) {
		nativeuser.Reset()
		_, err := New(
			t.Context(),
			"native-tool-missing",
			WithName("Missing Handler"),
			WithDescription("Fails"),
			WithRuntime("go"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "native handler")
	})

	t.Run("Should fail when native handler is nil", func(t *testing.T) {
		nativeuser.Reset()
		_, err := New(
			t.Context(),
			"native-tool-nil",
			WithName("Nil Handler"),
			WithDescription("Fails"),
			WithNativeHandler(nil),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "native handler")
	})
}

func TestNew_FullConfig(t *testing.T) {
	t.Run("Should create tool with all options", func(t *testing.T) {
		inputSchema := &engineschema.Schema{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
		}
		outputSchema := &engineschema.Schema{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{"type": "string"},
			},
		}
		withInput := &core.Input{"default_value": "test"}
		configInput := &core.Input{"api_url": "https://api.example.com"}
		envMap := &core.EnvMap{"API_KEY": "secret"}
		cfg, err := New(
			t.Context(),
			"full-tool",
			WithName("Full Tool"),
			WithDescription("A fully configured tool"),
			WithRuntime("bun"),
			WithCode("export default (input) => input"),
			WithTimeout("30s"),
			WithInputSchema(inputSchema),
			WithOutputSchema(outputSchema),
			WithWith(withInput),
			WithConfig(configInput),
			WithEnv(envMap),
		)
		require.NoError(t, err)
		assert.Equal(t, "full-tool", cfg.ID)
		assert.Equal(t, "Full Tool", cfg.Name)
		assert.Equal(t, "A fully configured tool", cfg.Description)
		assert.Equal(t, "bun", cfg.Runtime)
		assert.Equal(t, "export default (input) => input", cfg.Code)
		assert.Equal(t, "30s", cfg.Timeout)
		assert.NotNil(t, cfg.InputSchema)
		assert.NotNil(t, cfg.OutputSchema)
		assert.NotNil(t, cfg.With)
		assert.NotNil(t, cfg.Config)
		assert.NotNil(t, cfg.Env)
	})
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		opts    []Option
		wantErr string
	}{
		{
			name:    "Should fail with empty ID",
			id:      "",
			opts:    []Option{},
			wantErr: "id is invalid",
		},
		{
			name:    "Should fail with invalid ID",
			id:      "invalid id with spaces",
			opts:    []Option{},
			wantErr: "id is invalid",
		},
		{
			name: "Should fail with empty name",
			id:   "test-tool",
			opts: []Option{
				WithName(""),
			},
			wantErr: "tool name",
		},
		{
			name: "Should fail with whitespace-only name",
			id:   "test-tool",
			opts: []Option{
				WithName("   "),
			},
			wantErr: "tool name",
		},
		{
			name: "Should fail with empty description",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription(""),
			},
			wantErr: "tool description",
		},
		{
			name: "Should fail with whitespace-only description",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("   "),
			},
			wantErr: "tool description",
		},
		{
			name: "Should fail with empty runtime",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime(""),
			},
			wantErr: "tool runtime",
		},
		{
			name: "Should fail with invalid runtime",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("python"),
			},
			wantErr: "runtime must be bun",
		},
		{
			name: "Should fail with empty code",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode(""),
			},
			wantErr: "tool code",
		},
		{
			name: "Should fail with whitespace-only code",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode("   "),
			},
			wantErr: "tool code",
		},
		{
			name: "Should fail with invalid timeout format",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode("export default () => {}"),
				WithTimeout("invalid"),
			},
			wantErr: "invalid timeout format",
		},
		{
			name: "Should fail with negative timeout",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode("export default () => {}"),
				WithTimeout("-5s"),
			},
			wantErr: "timeout must be positive",
		},
		{
			name: "Should fail with zero timeout",
			id:   "test-tool",
			opts: []Option{
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode("export default () => {}"),
				WithTimeout("0s"),
			},
			wantErr: "timeout must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(t.Context(), tt.id, tt.opts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNew_NilContext(t *testing.T) {
	t.Run("Should fail with nil context", func(t *testing.T) {
		var nilCtx context.Context
		_, err := New(nilCtx, "test-tool")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context is required")
	})
}

func TestNew_DeepCopy(t *testing.T) {
	t.Run("Should return deep copied configuration", func(t *testing.T) {
		envMap := &core.EnvMap{"KEY": "value"}
		cfg1, err := New(
			t.Context(),
			"test-tool",
			WithName("Tool"),
			WithDescription("A tool"),
			WithRuntime("bun"),
			WithCode("export default () => {}"),
			WithEnv(envMap),
		)
		require.NoError(t, err)
		(*cfg1.Env)["KEY"] = "modified"
		cfg2, err := New(
			t.Context(),
			"test-tool",
			WithName("Tool"),
			WithDescription("A tool"),
			WithRuntime("bun"),
			WithCode("export default () => {}"),
			WithEnv(envMap),
		)
		require.NoError(t, err)
		assert.NotEqual(t, (*cfg1.Env)["KEY"], (*cfg2.Env)["KEY"])
		assert.Equal(t, "value", (*cfg2.Env)["KEY"])
	})
}

func TestNew_WhitespaceTrimming(t *testing.T) {
	t.Run("Should trim whitespace from all string fields", func(t *testing.T) {
		cfg, err := New(
			t.Context(),
			"  test-tool  ",
			WithName("  Test Tool  "),
			WithDescription("  A test tool  "),
			WithRuntime("  bun  "),
			WithCode("  export default () => {}  "),
		)
		require.NoError(t, err)
		assert.Equal(t, "test-tool", cfg.ID)
		assert.Equal(t, "Test Tool", cfg.Name)
		assert.Equal(t, "A test tool", cfg.Description)
		assert.Equal(t, "bun", cfg.Runtime)
		assert.Equal(t, "export default () => {}", cfg.Code)
	})
}

func TestNew_RuntimeCaseInsensitive(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
		want    string
	}{
		{
			name:    "Should normalize uppercase runtime",
			runtime: "BUN",
			want:    "bun",
		},
		{
			name:    "Should normalize mixed case runtime",
			runtime: "Bun",
			want:    "bun",
		},
		{
			name:    "Should keep lowercase runtime",
			runtime: "bun",
			want:    "bun",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(
				t.Context(),
				"test-tool",
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime(tt.runtime),
				WithCode("export default () => {}"),
			)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Runtime)
		})
	}
}

func TestNew_TimeoutParsing(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{
			name:    "Should accept seconds",
			timeout: "30s",
			wantErr: false,
		},
		{
			name:    "Should accept minutes",
			timeout: "5m",
			wantErr: false,
		},
		{
			name:    "Should accept hours",
			timeout: "1h",
			wantErr: false,
		},
		{
			name:    "Should accept milliseconds",
			timeout: "500ms",
			wantErr: false,
		},
		{
			name:    "Should accept combined durations",
			timeout: "1h30m",
			wantErr: false,
		},
		{
			name:    "Should accept no timeout",
			timeout: "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(
				t.Context(),
				"test-tool",
				WithName("Tool"),
				WithDescription("A tool"),
				WithRuntime("bun"),
				WithCode("export default () => {}"),
				WithTimeout(tt.timeout),
			)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.timeout, cfg.Timeout)
			}
		})
	}
}

func TestNew_SchemaValidation(t *testing.T) {
	t.Run("Should accept valid input schema", func(t *testing.T) {
		schema := &engineschema.Schema{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		}
		cfg, err := New(
			t.Context(),
			"test-tool",
			WithName("Tool"),
			WithDescription("A tool"),
			WithRuntime("bun"),
			WithCode("export default () => {}"),
			WithInputSchema(schema),
		)
		require.NoError(t, err)
		assert.NotNil(t, cfg.InputSchema)
		assert.Equal(t, "object", (*cfg.InputSchema)["type"])
	})
	t.Run("Should accept valid output schema", func(t *testing.T) {
		schema := &engineschema.Schema{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{"type": "string"},
			},
		}
		cfg, err := New(
			t.Context(),
			"test-tool",
			WithName("Tool"),
			WithDescription("A tool"),
			WithRuntime("bun"),
			WithCode("export default () => {}"),
			WithOutputSchema(schema),
		)
		require.NoError(t, err)
		assert.NotNil(t, cfg.OutputSchema)
		assert.Equal(t, "object", (*cfg.OutputSchema)["type"])
	})
}

func TestNew_MultipleErrors(t *testing.T) {
	t.Run("Should collect all validation errors", func(t *testing.T) {
		_, err := New(
			t.Context(),
			"",
			WithName(""),
			WithDescription(""),
			WithRuntime(""),
			WithCode(""),
		)
		require.Error(t, err)
		errStr := err.Error()
		assert.Contains(t, errStr, "id is invalid")
		assert.Contains(t, errStr, "tool name")
		assert.Contains(t, errStr, "tool description")
		assert.Contains(t, errStr, "tool runtime")
		assert.Contains(t, errStr, "tool code")
	})
}
