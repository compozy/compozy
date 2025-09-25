package orchestrator

import (
	"context"
	"errors"
	"testing"

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
}
