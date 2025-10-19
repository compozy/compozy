package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llm "github.com/compozy/compozy/engine/llm"
	adapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestrator "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/builtin/exec"
	"github.com/compozy/compozy/engine/tool/builtin/fetch"
	fsbuiltin "github.com/compozy/compozy/engine/tool/builtin/filesystem"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// registerBuiltinsInto creates builtin definitions and registers them into the provided registry.
func registerBuiltinsInto(ctx context.Context, t *testing.T, registry llm.ToolRegistry) *builtin.Result {
	t.Helper()
	fsDefs := fsbuiltin.Definitions()
	fetchDefs := fetch.Definitions()
	defs := make([]builtin.BuiltinDefinition, 0, len(fsDefs)+1+len(fetchDefs))
	defs = append(defs, fsDefs...)
	defs = append(defs, exec.Definition())
	defs = append(defs, fetchDefs...)

	registerFn := func(rctx context.Context, tool builtin.Tool) error {
		return registry.Register(rctx, tool)
	}
	res, err := builtin.RegisterBuiltins(ctx, registerFn, builtin.Options{Definitions: defs})
	require.NoError(t, err)
	return res
}

// newConfigContext builds a context carrying logger and app config with native tools setup.
func newConfigContext(t *testing.T, enabled bool, root string) context.Context {
	t.Helper()
	ctx := t.Context()
	ctx = logger.ContextWithLogger(ctx, logger.NewLogger(logger.TestConfig()))
	mgr := appconfig.NewManager(ctx, appconfig.NewService())
	_, err := mgr.Load(ctx, appconfig.NewDefaultProvider())
	require.NoError(t, err)
	cfg := mgr.Get()
	cfg.Runtime.NativeTools.Enabled = enabled
	if root != "" {
		cfg.Runtime.NativeTools.RootDir = root
	}
	return appconfig.ContextWithManager(ctx, mgr)
}

// orchestratorRegistryAdapter adapts llm.ToolRegistry to orchestrator.ToolRegistry.
type orchestratorRegistryAdapter struct{ registry llm.ToolRegistry }

func (a *orchestratorRegistryAdapter) Find(ctx context.Context, name string) (orchestrator.RegistryTool, bool) {
	t, ok := a.registry.Find(ctx, name)
	if !ok {
		return nil, false
	}
	return t, true
}

func (a *orchestratorRegistryAdapter) ListAll(ctx context.Context) ([]orchestrator.RegistryTool, error) {
	list, err := a.registry.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]orchestrator.RegistryTool, 0, len(list))
	for _, it := range list {
		out = append(out, it)
	}
	return out, nil
}

func (a *orchestratorRegistryAdapter) Close() error { return a.registry.Close() }

// executeSingle executes a single tool call using the orchestrator ToolExecutor against the given registry.
func executeSingle(ctx context.Context, registry llm.ToolRegistry, call adapter.ToolCall) (adapter.ToolResult, error) {
	toolExec := orchestrator.NewToolExecutor(&orchestratorRegistryAdapter{registry: registry}, nil)
	results, err := toolExec.Execute(ctx, []adapter.ToolCall{call})
	if err != nil {
		return adapter.ToolResult{}, err
	}
	if len(results) != 1 {
		return adapter.ToolResult{}, fmt.Errorf("expected 1 result, got %d", len(results))
	}
	return results[0], nil
}

func TestCpBuiltin_Integration(t *testing.T) {
	t.Run("Should register and execute cp__read_file when enabled", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		// Arrange: write a file inside the sandbox root
		fileRel := "hello.txt"
		fileAbs := filepath.Join(root, fileRel)
		require.NoError(t, os.WriteFile(fileAbs, []byte("hello world"), 0o644))

		ctx := newConfigContext(t, true, root)
		registry := llm.NewToolRegistry(t.Context(), llm.ToolRegistryConfig{})
		t.Cleanup(func() { _ = registry.Close() })
		res := registerBuiltinsInto(ctx, t, registry)
		require.NotEmpty(t, res.RegisteredIDs)
		assert.Contains(t, res.RegisteredIDs, "cp__read_file")
		assert.Contains(t, res.RegisteredIDs, "cp__write_file")

		// Act: call cp__read_file through the ToolExecutor
		args := map[string]any{"path": fileRel}
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		call := adapter.ToolCall{ID: "call_read", Name: "cp__read_file", Arguments: raw}
		result, err := executeSingle(ctx, registry, call)

		// Assert
		require.NoError(t, err)
		terr, ok := orchestrator.IsToolExecutionError(result.Content)
		assert.False(t, ok, "expected success, got orchestrator error: %+v", terr)
		require.NotEmpty(t, result.JSONContent)
		// Strongly-typed success payload with metadata checks
		type readMeta struct {
			Path     string `json:"path"`
			Size     int64  `json:"size"`
			Mode     int64  `json:"mode"`
			Modified string `json:"modified"`
		}
		type readFileResult struct {
			Content  string   `json:"content"`
			Metadata readMeta `json:"metadata"`
		}
		var payload readFileResult
		require.NoError(t, json.Unmarshal(result.JSONContent, &payload))
		assert.Equal(t, "hello world", payload.Content)
		assert.NotEmpty(t, payload.Metadata.Path)
		assert.Greater(t, payload.Metadata.Size, int64(0))
		assert.NotEmpty(t, payload.Metadata.Modified)
	})

	t.Run("Should return not-found when disabled via kill switch", func(t *testing.T) {
		t.Parallel()
		ctx := newConfigContext(t, false, t.TempDir())
		registry := llm.NewToolRegistry(t.Context(), llm.ToolRegistryConfig{})
		t.Cleanup(func() { _ = registry.Close() })
		res := registerBuiltinsInto(ctx, t, registry)
		assert.Empty(t, res.RegisteredIDs)

		call := adapter.ToolCall{ID: "call_nf", Name: "cp__read_file", Arguments: json.RawMessage(`{"path":"any.txt"}`)}
		result, err := executeSingle(ctx, registry, call)
		require.NoError(t, err)
		// ToolExecutor encodes not-found as a JSON payload; prefer JSONContent
		require.NotEmpty(t, result.JSONContent)
		var obj struct {
			Error string `json:"error"`
		}
		require.NoError(t, json.Unmarshal(result.JSONContent, &obj))
		require.NotEmpty(t, obj.Error)
		assert.Contains(t, obj.Error, "tool not found")
	})

	t.Run("Should map canonical FileNotFound error code", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ctx := newConfigContext(t, true, root)
		registry := llm.NewToolRegistry(t.Context(), llm.ToolRegistryConfig{})
		t.Cleanup(func() { _ = registry.Close() })
		_ = registerBuiltinsInto(ctx, t, registry)

		call := adapter.ToolCall{
			ID:        "call_missing",
			Name:      "cp__read_file",
			Arguments: json.RawMessage(`{"path":"missing.txt"}`),
		}
		result, err := executeSingle(ctx, registry, call)
		require.NoError(t, err)

		// Expect ToolExecutionResult with builtin.CodeFileNotFound
		terr, ok := orchestrator.IsToolExecutionError(result.Content)
		require.True(t, ok)
		assert.Equal(t, builtin.CodeFileNotFound, terr.Code)
	})

	t.Run("Should prevent path traversal with PermissionDenied", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ctx := newConfigContext(t, true, root)
		registry := llm.NewToolRegistry(t.Context(), llm.ToolRegistryConfig{})
		t.Cleanup(func() { _ = registry.Close() })
		_ = registerBuiltinsInto(ctx, t, registry)

		// Attempt to escape root via relative traversal
		call := adapter.ToolCall{
			ID:        "call_traverse",
			Name:      "cp__read_file",
			Arguments: json.RawMessage(`{"path":"../outside.txt"}`),
		}
		result, err := executeSingle(ctx, registry, call)
		require.NoError(t, err)
		terr, ok := orchestrator.IsToolExecutionError(result.Content)
		require.True(t, ok)
		assert.Equal(t, builtin.CodePermissionDenied, terr.Code)
	})

	t.Run("Should register and execute cp__write_file to create file", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		ctx := newConfigContext(t, true, root)
		registry := llm.NewToolRegistry(t.Context(), llm.ToolRegistryConfig{})
		t.Cleanup(func() { _ = registry.Close() })
		res := registerBuiltinsInto(ctx, t, registry)
		require.NotEmpty(t, res.RegisteredIDs)
		assert.Contains(t, res.RegisteredIDs, "cp__write_file")

		fileRel := "output.txt"
		content := "data written by tool"
		args := map[string]any{"path": fileRel, "content": content}
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		call := adapter.ToolCall{ID: "call_write", Name: "cp__write_file", Arguments: raw}
		result, err := executeSingle(ctx, registry, call)
		require.NoError(t, err)
		if terr, ok := orchestrator.IsToolExecutionError(result.Content); ok {
			t.Fatalf("unexpected tool error: %+v", terr)
		}
		data, err := os.ReadFile(filepath.Join(root, fileRel))
		require.NoError(t, err)
		assert.Equal(t, content, string(data))
	})
}
