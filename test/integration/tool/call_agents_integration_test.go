package tool

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/resources"
	_ "github.com/compozy/compozy/engine/tool/builtin/imports"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestCallAgentsParallelIntegration(t *testing.T) {
	t.Run("Should execute cp__call_agents via llm service", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		project := "demo"
		ctx = core.WithProjectName(ctx, project)
		manager := config.NewManager(t.Context(), config.NewService())
		_, err := manager.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Runtime.NativeTools.Enabled = true
		cfg.Runtime.NativeTools.CallAgent.Enabled = true
		cfg.Runtime.NativeTools.CallAgents.Enabled = true
		cfg.Runtime.NativeTools.CallAgents.MaxConcurrent = 5
		ctx = config.ContextWithManager(ctx, manager)
		executor := &recordingAgentExecutor{}
		store := resources.NewMemoryResourceStore()
		seedAgent(t, store, project, map[string]any{
			"actions": []any{map[string]any{"id": "summarize", "prompt": "Summarize"}},
		})
		_, err = store.Put(ctx, resources.ResourceKey{
			Project: project,
			Type:    resources.ResourceAgent,
			ID:      "agent.research",
		}, map[string]any{
			"actions": []any{map[string]any{"id": "research", "prompt": "Research"}},
		})
		require.NoError(t, err)
		env := &staticEnvironment{executor: executor, store: store}
		callPayload := map[string]any{
			"agents": []any{
				map[string]any{
					"agent_id":  "agent.summary",
					"action_id": "summarize",
					"with":      map[string]any{"topic": "distributed systems"},
				},
				map[string]any{
					"agent_id": "agent.research",
					"prompt":   "Collect industry reports",
				},
			},
		}
		callArgs, err := json.Marshal(callPayload)
		require.NoError(t, err)
		script := newScriptedLLM([]*llmadapter.LLMResponse{
			{
				Content: "",
				ToolCalls: []llmadapter.ToolCall{
					{ID: "call-agents", Name: "cp__call_agents", Arguments: callArgs},
				},
			},
			{Content: "Parallel execution complete"},
		})
		factory := scriptedFactory{client: script}
		agentCfg := CreateTestAgentConfig(nil)
		runtimeMgr := CreateMockRuntime(ctx, t)
		service, err := llm.NewService(ctx, runtimeMgr, agentCfg,
			llm.WithLLMFactory(factory),
			llm.WithToolEnvironment(env),
			llm.WithTimeout(5*time.Second),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = service.Close() })
		output, err := service.GenerateContent(ctx, agentCfg, nil, "", "coordinate parallel agents", nil)
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, "Parallel execution complete", output.Prop("response"))
		require.Equal(t, 2, executor.Count())
		requests := script.Requests()
		require.Len(t, requests, 2)
		result, ok := findToolResult(requests[1].Messages, "cp__call_agents")
		require.True(t, ok)
		require.NotEmpty(t, result.JSONContent)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(result.JSONContent, &payload))
		assert.Equal(t, float64(2), payload["total_count"])
		assert.Equal(t, float64(2), payload["success_count"])
		results, ok := payload["results"].([]any)
		require.True(t, ok)
		require.Len(t, results, 2)
		firstResult, ok := results[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "agent.summary", firstResult["agent_id"])
		secondResult, ok := results[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "agent.research", secondResult["agent_id"])
	})
}
