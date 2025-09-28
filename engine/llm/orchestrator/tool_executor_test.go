package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubToolRegistry struct {
	tools map[string]RegistryTool
}

func newStubToolRegistry() *stubToolRegistry {
	return &stubToolRegistry{tools: make(map[string]RegistryTool)}
}

func (s *stubToolRegistry) Find(_ context.Context, name string) (RegistryTool, bool) {
	t, ok := s.tools[name]
	return t, ok
}

func (s *stubToolRegistry) ListAll(context.Context) ([]RegistryTool, error) {
	values := make([]RegistryTool, 0, len(s.tools))
	for _, tool := range s.tools {
		values = append(values, tool)
	}
	return values, nil
}

func (s *stubToolRegistry) Close() error { return nil }

func (s *stubToolRegistry) register(t RegistryTool) {
	s.tools[t.Name()] = t
}

type fnTool struct {
	name        string
	description string
	call        func(ctx context.Context, input string) (string, error)
}

func (f *fnTool) Name() string        { return f.name }
func (f *fnTool) Description() string { return f.description }
func (f *fnTool) Call(ctx context.Context, input string) (string, error) {
	return f.call(ctx, input)
}

func TestToolExecutor_Execute(t *testing.T) {
	t.Run("Should populate JSONContent when tool returns JSON", func(t *testing.T) {
		registry := newStubToolRegistry()
		registry.register(&fnTool{
			name:        "json-tool",
			description: "",
			call: func(_ context.Context, _ string) (string, error) {
				return `{"ok":true}`, nil
			},
		})

		exec := NewToolExecutor(registry, &settings{maxConcurrentTools: 1})
		ctx := context.Background()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "json-tool", Arguments: []byte(`{"arg":1}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.JSONEq(t, `{"ok":true}`, string(results[0].JSONContent))
	})

	t.Run("Should return structured error when tool missing", func(t *testing.T) {
		exec := NewToolExecutor(newStubToolRegistry(), &settings{maxConcurrentTools: 1})
		ctx := context.Background()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "missing", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Contains(t, string(results[0].JSONContent), "tool not found: missing")
	})

	t.Run("Should propagate execution errors as JSON payload", func(t *testing.T) {
		registry := newStubToolRegistry()
		registry.register(&fnTool{
			name:        "fail-tool",
			description: "",
			call: func(_ context.Context, _ string) (string, error) {
				return "", errors.New("boom")
			},
		})

		exec := NewToolExecutor(registry, &settings{maxConcurrentTools: 1})
		ctx := context.Background()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "fail-tool", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Content, "Tool execution failed")
		assert.Contains(t, string(results[0].JSONContent), "TOOL_EXECUTION_ERROR")
	})

	t.Run("Should write tool input and output to log file", func(t *testing.T) {
		registry := newStubToolRegistry()
		registry.register(&fnTool{
			name:        "echo",
			description: "",
			call: func(_ context.Context, input string) (string, error) {
				return "processed:" + input, nil
			},
		})
		root := t.TempDir()
		exec := NewToolExecutor(registry, &settings{maxConcurrentTools: 1, projectRoot: root})
		ctx := context.Background()
		_, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "echo", Arguments: []byte(`{"hello":"world"}`)},
		})
		require.NoError(t, err)
		logPath := filepath.Join(core.GetStoreDir(root), "tools_log.json")
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		dec := json.NewDecoder(bytes.NewReader(data))
		var entries []map[string]any
		for {
			var entry map[string]any
			err := dec.Decode(&entry)
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			entries = append(entries, entry)
		}
		require.Len(t, entries, 1)
		entry := entries[0]
		assert.Equal(t, "call-1", entry["tool_call_id"])
		assert.Equal(t, "echo", entry["tool_name"])
		assert.Equal(t, "success", entry["status"])
		assert.NotEmpty(t, entry["timestamp"])
		inputVal, ok := entry["input"].(string)
		require.True(t, ok)
		assert.Contains(t, inputVal, "\"hello\":\"world\"")
		outputVal, ok := entry["output"].(string)
		require.True(t, ok)
		assert.Equal(t, "processed:{\"hello\":\"world\"}", outputVal)
	})
}
