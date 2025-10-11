package orchestrator

import (
	"encoding/json"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

func TestContainsSuccessfulAgentCall(t *testing.T) {
	t.Run("ShouldReturnTrueWhenAgentCallSucceeds", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__call_agent",
				Content: `{"agent_id":"researcher","success":true}`,
			},
		}
		require.True(t, containsSuccessfulAgentCall(results))
	})
	t.Run("ShouldReturnFalseWhenAgentCallFails", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__call_agent",
				Content: `{"error":"agent not found"}`,
			},
		}
		require.False(t, containsSuccessfulAgentCall(results))
	})
	t.Run("ShouldReturnFalseWhenNoAgentCalls", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__read_file",
				Content: `{"content":"test"}`,
			},
		}
		require.False(t, containsSuccessfulAgentCall(results))
	})
	t.Run("ShouldReturnTrueWhenMixedToolsWithSuccessfulAgentCall", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__read_file",
				Content: `{"content":"test"}`,
			},
			{
				Name:    "cp__call_agent",
				Content: `{"agent_id":"researcher","response":{"answer":"ok"}}`,
			},
		}
		require.True(t, containsSuccessfulAgentCall(results))
	})
}

func TestBuildAgentCallCompletionHint(t *testing.T) {
	t.Run("ShouldBuildHintForSingleAgent", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__call_agent",
				Content: `{"agent_id":"researcher","success":true}`,
			},
		}
		hint := buildAgentCallCompletionHint(results)
		require.Contains(t, hint, "'researcher'")
		require.Contains(t, hint, "successfully completed its task")
		require.Contains(t, hint, "provide your final answer")
	})
	t.Run("ShouldBuildHintForMultipleAgents", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__call_agent",
				Content: `{"agent_id":"researcher","success":true}`,
			},
			{
				Name:    "cp__call_agent",
				Content: `{"agent_id":"writer","success":true}`,
			},
		}
		hint := buildAgentCallCompletionHint(results)
		require.Contains(t, hint, "'researcher'")
		require.Contains(t, hint, "'writer'")
		require.Contains(t, hint, "agents")
		require.Contains(t, hint, "have successfully completed their tasks")
	})
	t.Run("ShouldReturnEmptyWhenNoAgentCalls", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__read_file",
				Content: `{"content":"test"}`,
			},
		}
		hint := buildAgentCallCompletionHint(results)
		require.Empty(t, hint)
	})
	t.Run("ShouldIgnoreFailedAgentCalls", func(t *testing.T) {
		results := []llmadapter.ToolResult{
			{
				Name:    "cp__call_agent",
				Content: `{"error":"agent not found"}`,
			},
		}
		hint := buildAgentCallCompletionHint(results)
		require.Empty(t, hint)
	})
}

func TestExtractAgentIDFromResult(t *testing.T) {
	t.Run("ShouldExtractFromJSONContent", func(t *testing.T) {
		result := llmadapter.ToolResult{
			JSONContent: json.RawMessage(`{"agent_id":"researcher"}`),
		}
		agentID := extractAgentIDFromResult(result)
		require.Equal(t, "researcher", agentID)
	})
	t.Run("ShouldExtractFromContentString", func(t *testing.T) {
		result := llmadapter.ToolResult{
			Content: `{"agent_id":"writer"}`,
		}
		agentID := extractAgentIDFromResult(result)
		require.Equal(t, "writer", agentID)
	})
	t.Run("ShouldReturnEmptyForInvalidJSON", func(t *testing.T) {
		result := llmadapter.ToolResult{
			Content: "not json",
		}
		agentID := extractAgentIDFromResult(result)
		require.Empty(t, agentID)
	})
	t.Run("ShouldReturnEmptyWhenAgentIDMissing", func(t *testing.T) {
		result := llmadapter.ToolResult{
			Content: `{"success":true}`,
		}
		agentID := extractAgentIDFromResult(result)
		require.Empty(t, agentID)
	})
}
