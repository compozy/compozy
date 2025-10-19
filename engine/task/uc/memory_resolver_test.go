package uc

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 5 * time.Second

func testContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	ctx = config.ContextWithManager(ctx, config.NewManager(t.Context(), config.NewService()))
	return ctx, cancel
}

func TestNewMemoryResolver(t *testing.T) {
	t.Run("Should create memory resolver", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowCtx := map[string]any{"workflow": map[string]any{"id": "test-workflow"}}
		resolver := NewMemoryResolver(nil, tplEngine, workflowCtx)
		assert.NotNil(t, resolver)
		assert.Nil(t, resolver.memoryManager)
		assert.Equal(t, tplEngine, resolver.templateEngine)
		assert.Equal(t, workflowCtx, resolver.workflowContext)
	})
}

func TestMemoryResolver_GetMemory(t *testing.T) {
	ctx, cancel := testContext(t)
	defer cancel()
	t.Run("Should return nil when memory manager is nil", func(t *testing.T) {
		resolver := &MemoryResolver{memoryManager: nil}
		memory, err := resolver.GetMemory(ctx, "test-memory", "key-template")
		assert.NoError(t, err)
		assert.Nil(t, memory)
	})
	t.Run("Should return nil when memory ID is empty", func(t *testing.T) {
		resolver := &MemoryResolver{memoryManager: nil}
		memory, err := resolver.GetMemory(ctx, " ", "key-template")
		assert.NoError(t, err)
		assert.Nil(t, memory)
	})
}

func TestMemoryResolver_ResolveAgentMemories(t *testing.T) {
	ctx, cancel := testContext(t)
	defer cancel()
	t.Run("Should return nil map when agent has no memory references", func(t *testing.T) {
		resolver := &MemoryResolver{memoryManager: nil}
		agentCfg := &agent.Config{ID: "test-agent"}
		memories, err := resolver.ResolveAgentMemories(ctx, agentCfg)
		assert.NoError(t, err)
		assert.Nil(t, memories)
	})
	t.Run("Should skip read-only memories", func(t *testing.T) {
		resolver := &MemoryResolver{memoryManager: nil}
		agentCfg := &agent.Config{
			ID: "test-agent",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{{ID: "ro-memory", Mode: core.MemoryModeReadOnly}},
			},
		}
		memories, err := resolver.ResolveAgentMemories(ctx, agentCfg)
		assert.NoError(t, err)
		assert.NotNil(t, memories)
		assert.Empty(t, memories)
	})
}

func TestMemoryResolverTemplateResolution(t *testing.T) {
	ctx, cancel := testContext(t)
	defer cancel()
	t.Run("Should resolve nested context templates", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"workflow": map[string]any{
				"id":      "wf-123",
				"exec_id": "exec-456",
				"input": map[string]any{
					"user_id":    "user789",
					"session_id": "sess101",
					"metadata": map[string]any{
						"channel": "web",
						"version": "v2.1",
					},
				},
			},
			"project": map[string]any{"id": "proj-abc"},
			"task":    map[string]any{"type": "basic"},
		}
		resolver := &MemoryResolver{templateEngine: tplEngine, workflowContext: workflowContext}
		testCases := []struct {
			template string
			expected string
		}{
			{"{{.workflow.id}}:{{.project.id}}:{{.workflow.input.user_id}}", "wf-123:proj-abc:user789"},
			{"{{.workflow.input.metadata.channel}}-{{.workflow.input.metadata.version}}", "web-v2.1"},
			{"session:{{.workflow.input.session_id}}:{{.task.type}}", "session:sess101:basic"},
		}
		for _, tc := range testCases {
			resolved, err := resolver.resolveKey(ctx, tc.template)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, resolved)
		}
	})
	t.Run("Should handle edge cases in templates", func(t *testing.T) {
		tplEngine := tplengine.NewEngine(tplengine.FormatText)
		workflowContext := map[string]any{
			"empty_string": "",
			"zero_value":   0,
			"null_value":   nil,
			"bool_value":   true,
		}
		resolver := &MemoryResolver{templateEngine: tplEngine, workflowContext: workflowContext}
		testCases := []struct {
			template string
			expected string
		}{
			{"empty:{{.empty_string}}", "empty:"},
			{"zero:{{.zero_value}}", "zero:0"},
			{"bool:{{.bool_value}}", "bool:true"},
		}
		for _, tc := range testCases {
			resolved, err := resolver.resolveKey(ctx, tc.template)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, resolved)
		}
		resolved, err := resolver.resolveKey(ctx, "null:{{.null_value}}")
		require.NoError(t, err)
		assert.Equal(t, "null:<no value>", resolved)
	})
	t.Run("Should error when template engine missing for templated key", func(t *testing.T) {
		resolver := &MemoryResolver{templateEngine: nil, workflowContext: map[string]any{}}
		resolved, err := resolver.resolveKey(ctx, "value-{{.missing}}")
		assert.Error(t, err)
		assert.Empty(t, resolved)
	})
}
