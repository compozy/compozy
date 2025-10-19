package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/tool/builtin"
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
	params      map[string]any
}

func (f *fnTool) Name() string        { return f.name }
func (f *fnTool) Description() string { return f.description }
func (f *fnTool) Call(ctx context.Context, input string) (string, error) {
	return f.call(ctx, input)
}
func (f *fnTool) ParameterSchema() map[string]any { return f.params }

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
		ctx := t.Context()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "json-tool", Arguments: []byte(`{"arg":1}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.JSONEq(t, `{"ok":true}`, string(results[0].JSONContent))
	})

	t.Run("Should return structured error when tool missing", func(t *testing.T) {
		exec := NewToolExecutor(newStubToolRegistry(), &settings{maxConcurrentTools: 1})
		ctx := t.Context()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "missing", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Contains(t, string(results[0].JSONContent), "tool not found: missing")
	})

	t.Run("Should validate tool arguments before execution", func(t *testing.T) {
		called := false
		registry := newStubToolRegistry()
		registry.register(&fnTool{
			name:        "validator",
			description: "",
			params: map[string]any{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
			call: func(_ context.Context, input string) (string, error) {
				called = true
				return input, nil
			},
		})

		exec := NewToolExecutor(registry, &settings{maxConcurrentTools: 1})
		ctx := t.Context()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "validator", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, called, "tool should not execute when validation fails")
		assert.Contains(t, results[0].Content, "Invalid tool arguments")
		assert.Contains(t, string(results[0].JSONContent), ErrCodeToolInvalidInput)
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
		ctx := t.Context()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "fail-tool", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Contains(t, results[0].Content, "Tool execution failed")
		assert.Contains(t, string(results[0].JSONContent), "TOOL_EXECUTION_ERROR")
	})

	t.Run("Should propagate remediation hints from structured errors", func(t *testing.T) {
		registry := newStubToolRegistry()
		registry.register(&fnTool{
			name:        "hint-tool",
			description: "",
			call: func(_ context.Context, _ string) (string, error) {
				return "", builtin.InvalidArgument(
					fmt.Errorf("planner produced invalid plan: plan requires at least one step"),
					map[string]any{"remediation": "Add at least one plan step before invoking the tool again."},
				)
			},
		})
		exec := NewToolExecutor(registry, &settings{maxConcurrentTools: 1})
		ctx := t.Context()
		results, err := exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "hint-tool", Arguments: []byte(`{}`)},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(results[0].JSONContent, &payload))
		require.False(t, payload["success"].(bool))
		errSection, ok := payload["error"].(map[string]any)
		require.True(t, ok)
		require.Equal(
			t,
			"Add at least one plan step before invoking the tool again.",
			errSection["remediation_hint"],
		)
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
		recorder, err := telemetry.NewRecorder(&telemetry.Options{ProjectRoot: root})
		require.NoError(t, err)

		ctx, run, err := recorder.StartRun(t.Context(), telemetry.RunMetadata{})
		require.NoError(t, err)
		defer func() {
			_ = recorder.CloseRun(ctx, run, telemetry.RunResult{Success: true})
		}()

		_, err = exec.Execute(ctx, []llmadapter.ToolCall{
			{ID: "call-1", Name: "echo", Arguments: []byte(`{"hello":"world"}`)},
		})
		require.NoError(t, err)
		logPath := filepath.Join(core.GetStoreDir(root), "tools_log.ndjson")
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
		require.Len(t, lines, 1)
		var entry map[string]any
		require.NoError(t, json.Unmarshal(lines[0], &entry))
		assert.Equal(t, "call-1", entry["tool_call_id"])
		assert.Equal(t, "echo", entry["tool_name"])
		assert.Equal(t, "success", entry["status"])
		assert.NotEmpty(t, entry["timestamp"])
		assert.Contains(t, entry["input"], "\"hello\":\"world\"")
		assert.Equal(t, "processed:{\"hello\":\"world\"}", entry["output"])
	})
}

func TestToolExecutor_UpdateBudgets_ErrorBudgetExceeded(t *testing.T) {
	t.Run("Should error when tool error budget exceeded", func(t *testing.T) {
		exec := NewToolExecutor(newStubToolRegistry(), &settings{maxSequentialToolErrors: 2})
		st := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
		results := []llmadapter.ToolResult{{Name: "t", Content: `{"error":"x"}`}, {Name: "t", Content: `{"error":"x"}`}}
		err := exec.UpdateBudgets(t.Context(), results, st)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBudgetExceeded)
		assert.ErrorContains(t, err, "tool error budget exceeded for t")
	})
}

func TestToolExecutor_UpdateBudgets_ConsecutiveSuccessExceeded(t *testing.T) {
	t.Run("Should error on consecutive successes without progress", func(t *testing.T) {
		exec := NewToolExecutor(
			newStubToolRegistry(),
			&settings{maxConsecutiveSuccesses: 2, enableProgressTracking: true},
		)
		st := newLoopState(&settings{maxConsecutiveSuccesses: 2, enableProgressTracking: true}, nil, nil)
		results := []llmadapter.ToolResult{
			{Name: "t", JSONContent: []byte(`{"ok":true}`)},
			{Name: "t", JSONContent: []byte(`{"ok":true}`)},
		}
		err := exec.UpdateBudgets(t.Context(), results, st)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBudgetExceeded)
		assert.ErrorContains(t, err, "tool t called successfully 2 times without progress")
	})

	t.Run("Should reset success counter when tool output changes", func(t *testing.T) {
		exec := NewToolExecutor(newStubToolRegistry(), &settings{maxConsecutiveSuccesses: 2})
		st := newLoopState(&settings{maxConsecutiveSuccesses: 2}, nil, nil)
		results := []llmadapter.ToolResult{
			{Name: "t", JSONContent: []byte(`{"file":"a"}`)},
			{Name: "t", JSONContent: []byte(`{"file":"b"}`)},
			{Name: "t", JSONContent: []byte(`{"file":"c"}`)},
		}
		err := exec.UpdateBudgets(t.Context(), results, st)
		require.NoError(t, err)
	})

	t.Run("Should still respect success limit when output repeats without progress", func(t *testing.T) {
		exec := NewToolExecutor(newStubToolRegistry(), &settings{maxConsecutiveSuccesses: 2})
		st := newLoopState(&settings{maxConsecutiveSuccesses: 2}, nil, nil)
		results := []llmadapter.ToolResult{
			{Name: "t", Content: "first"},
			{Name: "t", Content: "first"},
		}
		err := exec.UpdateBudgets(t.Context(), results, st)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBudgetExceeded)
		assert.ErrorContains(t, err, "tool t called successfully 2 times without progress")
	})
}

func TestToolExecutor_UpdateBudgets_ToolCapExceeded(t *testing.T) {
	toolCaps := toolCallCaps{defaultLimit: 1}
	exec := NewToolExecutor(newStubToolRegistry(), &settings{toolCaps: toolCaps})
	st := newLoopState(&settings{toolCaps: toolCaps, finalizeOutputRetries: 1}, nil, nil)
	results := []llmadapter.ToolResult{{Name: "search", JSONContent: []byte(`{"ok":true}`)}}
	require.NoError(t, exec.UpdateBudgets(t.Context(), results, st))
	err := exec.UpdateBudgets(t.Context(), results, st)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBudgetExceeded)
	assert.ErrorContains(t, err, "invocation cap exceeded")
}
