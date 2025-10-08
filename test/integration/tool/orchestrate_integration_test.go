package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	_ "github.com/compozy/compozy/engine/tool/builtin/orchestrate"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestAgentOrchestrateIntegration(t *testing.T) {
	t.Run("Should execute cp__agent_orchestrate via llm service", func(t *testing.T) {
		ctx := logger.ContextWithLogger(context.Background(), logger.NewLogger(logger.TestConfig()))
		manager := config.NewManager(config.NewService())
		_, err := manager.Load(ctx, config.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Runtime.NativeTools.Enabled = true
		cfg.Runtime.NativeTools.AgentOrchestrator.Enabled = true
		cfg.Runtime.NativeTools.AgentOrchestrator.MaxParallel = 2
		cfg.Runtime.NativeTools.AgentOrchestrator.MaxSteps = 4
		ctx = config.ContextWithManager(ctx, manager)
		executor := &recordingAgentExecutor{}
		env := &staticEnvironment{executor: executor}
		firstCallPayload := map[string]any{
			"plan": map[string]any{
				"id": "plan-alpha",
				"steps": []any{
					map[string]any{
						"id":     "step-agent",
						"type":   "agent",
						"status": "pending",
						"agent": map[string]any{
							"agent_id":   "agent.summary",
							"result_key": "summary",
						},
					},
				},
			},
		}
		firstCallArgs, err := json.Marshal(firstCallPayload)
		require.NoError(t, err)
		script := newScriptedLLM([]*llmadapter.LLMResponse{
			{
				// Content unused; empty string indicates tool-only response.
				Content: "",
				ToolCalls: []llmadapter.ToolCall{
					{ID: "call-orchestrate", Name: "cp__agent_orchestrate", Arguments: firstCallArgs},
				},
			},
			{
				Content: "Orchestration complete",
			},
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
		output, err := service.GenerateContent(ctx, agentCfg, nil, "", "Coordinate multi-agent workflow", nil)
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, "Orchestration complete", output.Prop("response"))
		requests := script.Requests()
		require.Equal(t, 1, executor.Count())
		lastReq, ok := executor.LastRequest()
		require.True(t, ok)
		assert.Equal(t, "agent.summary", lastReq.AgentID)
		require.Len(t, requests, 2)
		result, ok := findToolResult(requests[1].Messages, "cp__agent_orchestrate")
		require.True(t, ok)
		require.NotEmpty(t, result.JSONContent)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(result.JSONContent, &payload))
		assert.Equal(t, true, payload["success"])
		steps, ok := payload["steps"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, steps)
	})
}

type scriptedFactory struct {
	client *scriptedLLM
}

func (f scriptedFactory) CreateClient(context.Context, *core.ProviderConfig) (llmadapter.LLMClient, error) {
	if f.client == nil {
		return nil, fmt.Errorf("nil scripted client")
	}
	return f.client, nil
}

type scriptedLLM struct {
	mu        sync.Mutex
	responses []*llmadapter.LLMResponse
	requests  []llmadapter.LLMRequest
}

func newScriptedLLM(responses []*llmadapter.LLMResponse) *scriptedLLM {
	return &scriptedLLM{responses: responses, requests: make([]llmadapter.LLMRequest, 0, len(responses))}
}

func (s *scriptedLLM) GenerateContent(_ context.Context, req *llmadapter.LLMRequest) (*llmadapter.LLMResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := cloneLLMRequest(req)
	s.requests = append(s.requests, clone)
	if len(s.responses) == 0 {
		return nil, fmt.Errorf("unexpected llm invocation")
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func (s *scriptedLLM) Close() error {
	return nil
}

func (s *scriptedLLM) Requests() []llmadapter.LLMRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]llmadapter.LLMRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

type recordingAgentExecutor struct {
	mu    sync.Mutex
	calls []toolenv.AgentRequest
}

func (r *recordingAgentExecutor) ExecuteAgent(
	_ context.Context,
	req toolenv.AgentRequest,
) (*toolenv.AgentResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	r.mu.Unlock()
	output := core.Output{
		"agent_id": req.AgentID,
	}
	return &toolenv.AgentResult{
		ExecID: core.MustNewID(),
		Output: &output,
	}, nil
}

func (r *recordingAgentExecutor) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recordingAgentExecutor) LastRequest() (toolenv.AgentRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return toolenv.AgentRequest{}, false
	}
	return r.calls[len(r.calls)-1], true
}

type staticEnvironment struct {
	executor toolenv.AgentExecutor
}

func (e *staticEnvironment) AgentExecutor() toolenv.AgentExecutor {
	return e.executor
}

func (e *staticEnvironment) TaskRepository() task.Repository {
	return nil
}

func (e *staticEnvironment) ResourceStore() resources.ResourceStore {
	return nil
}

func cloneLLMRequest(req *llmadapter.LLMRequest) llmadapter.LLMRequest {
	msgs := make([]llmadapter.Message, len(req.Messages))
	for i := range req.Messages {
		msg := req.Messages[i]
		msg.ToolCalls = cloneToolCalls(msg.ToolCalls)
		msg.ToolResults = cloneToolResults(msg.ToolResults)
		msgs[i] = msg
	}
	tools := make([]llmadapter.ToolDefinition, len(req.Tools))
	copy(tools, req.Tools)
	return llmadapter.LLMRequest{
		SystemPrompt: req.SystemPrompt,
		Messages:     msgs,
		Tools:        tools,
		Options:      req.Options,
	}
}

func cloneToolCalls(calls []llmadapter.ToolCall) []llmadapter.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	cloned := make([]llmadapter.ToolCall, len(calls))
	for i := range calls {
		raw := make([]byte, len(calls[i].Arguments))
		copy(raw, calls[i].Arguments)
		cloned[i] = llmadapter.ToolCall{
			ID:        calls[i].ID,
			Name:      calls[i].Name,
			Arguments: raw,
		}
	}
	return cloned
}

func cloneToolResults(results []llmadapter.ToolResult) []llmadapter.ToolResult {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]llmadapter.ToolResult, len(results))
	for i := range results {
		raw := make([]byte, len(results[i].JSONContent))
		copy(raw, results[i].JSONContent)
		cloned[i] = llmadapter.ToolResult{
			ID:          results[i].ID,
			Name:        results[i].Name,
			Content:     results[i].Content,
			JSONContent: raw,
		}
	}
	return cloned
}

func findToolResult(messages []llmadapter.Message, toolName string) (*llmadapter.ToolResult, bool) {
	for i := range messages {
		if messages[i].Role != llmadapter.RoleTool {
			continue
		}
		for j := range messages[i].ToolResults {
			if messages[i].ToolResults[j].Name == toolName {
				return &messages[i].ToolResults[j], true
			}
		}
	}
	return nil, false
}
