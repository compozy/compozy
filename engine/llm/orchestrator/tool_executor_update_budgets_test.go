package orchestrator

import (
	"context"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolExecutor_UpdateBudgets_ErrorBudgetExceeded(t *testing.T) {
	exec := NewToolExecutor(newStubToolRegistry(), &settings{maxSequentialToolErrors: 2})
	st := newLoopState(&settings{maxSequentialToolErrors: 2}, nil, nil)
	results := []llmadapter.ToolResult{{Name: "t", Content: `{"error":"x"}`}, {Name: "t", Content: `{"error":"x"}`}}
	err := exec.UpdateBudgets(context.Background(), results, st)
	require.Error(t, err)
	assert.ErrorContains(t, err, "tool error budget exceeded for t")
}

func TestToolExecutor_UpdateBudgets_ConsecutiveSuccessExceeded(t *testing.T) {
	exec := NewToolExecutor(newStubToolRegistry(), &settings{maxConsecutiveSuccesses: 2, enableProgressTracking: true})
	st := newLoopState(&settings{maxConsecutiveSuccesses: 2, enableProgressTracking: true}, nil, nil)
	results := []llmadapter.ToolResult{
		{Name: "t", JSONContent: []byte(`{"ok":true}`)},
		{Name: "t", JSONContent: []byte(`{"ok":true}`)},
	}
	err := exec.UpdateBudgets(context.Background(), results, st)
	require.Error(t, err)
	assert.ErrorContains(t, err, "tool t called successfully 2 times without progress")
}
