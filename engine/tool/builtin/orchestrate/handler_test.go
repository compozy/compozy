package orchestrate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	t.Run("Should execute structured plan", func(t *testing.T) {
		ctx, env := newHandlerContext(t)
		handler := Definition(env).Handler
		payload := map[string]any{
			"plan": map[string]any{
				"steps": []any{
					map[string]any{
						"id":     "step_1",
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
		output, err := handler(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, output)
		value, exists := output["success"]
		if !exists {
			t.Fatalf("missing success key output=%v", output)
		}
		success, ok := value.(bool)
		require.Truef(t, ok, "success type=%T value=%v output=%v", value, value, output)
		require.Truef(t, success, "output=%v", output)
		steps, ok := output["steps"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, steps, 1)
		assert.Equal(t, "step_1", steps[0]["id"])
		assert.Equal(t, "agent.summary", env.exec.lastReq.AgentID)
		assert.Equal(t, 1, env.exec.callCount)
		bindingsValue, bindingsExists := output["bindings"]
		require.Truef(t, bindingsExists, "output=%v", output)
		bindings, ok := bindingsValue.(map[string]any)
		require.Truef(t, ok, "bindings type=%T value=%v", bindingsValue, bindingsValue)
		result, ok := bindings["summary"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "agent.summary", result["agent"])
	})

	t.Run("Should compile prompt into plan", func(t *testing.T) {
		ctx, env := newHandlerContext(t)
		adapter := llmadapter.NewTestAdapter()
		planJSON := map[string]any{
			"id": "prompt-plan",
			"steps": []any{
				map[string]any{
					"id":     "step_1",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id":   "agent.generate",
						"result_key": "generated",
					},
				},
			},
		}
		bytes, err := json.Marshal(planJSON)
		require.NoError(t, err)
		adapter.SetResponse(string(bytes))
		ctx = llmadapter.ContextWithClient(ctx, testLLMClient{TestAdapter: adapter})
		handler := Definition(env).Handler
		payload := map[string]any{
			"prompt": "Generate content with agent.generate",
			"bindings": map[string]any{
				"topic": "testing",
			},
		}
		output, err := handler(ctx, payload)
		require.NoError(t, err)
		success, ok := output["success"].(bool)
		require.True(t, ok)
		require.Truef(t, success, "output=%v", output)
		assert.Equal(t, "agent.generate", env.exec.lastReq.AgentID)
	})

	t.Run("Should reject prompt when planner disabled", func(t *testing.T) {
		ctx, env := newHandlerContext(t)
		cfg := env.manager.Get()
		cfg.Runtime.NativeTools.AgentOrchestrator.Planner.Disabled = true
		handler := Definition(env).Handler
		_, err := handler(ctx, map[string]any{"prompt": "Plan this"})
		require.Error(t, err)
	})
	t.Run("Should expose definition through native catalog", func(t *testing.T) {
		_, env := newHandlerContext(t)
		defs := native.Definitions(env)
		found := false
		for i := range defs {
			if defs[i].ID == toolID {
				found = true
			}
		}
		require.True(t, found)
	})
}

type handlerTestEnv struct {
	exec    *stubAgentExecutor
	manager *config.Manager
}

var _ toolenv.Environment = (*handlerTestEnv)(nil)

type testLLMClient struct{ *llmadapter.TestAdapter }

func (c testLLMClient) Close() error { return nil }

func newHandlerContext(t *testing.T) (context.Context, *handlerTestEnv) {
	t.Helper()
	ctx := logger.ContextWithLogger(context.Background(), logger.NewLogger(logger.TestConfig()))
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.AgentOrchestrator.Enabled = true
	cfg.Runtime.NativeTools.AgentOrchestrator.MaxParallel = 4
	cfg.Runtime.NativeTools.AgentOrchestrator.MaxSteps = 5
	ctx = config.ContextWithManager(ctx, manager)
	exec := &stubAgentExecutor{}
	env := &handlerTestEnv{
		exec:    exec,
		manager: manager,
	}
	return ctx, env
}

func (e *handlerTestEnv) AgentExecutor() toolenv.AgentExecutor {
	return e.exec
}

func (e *handlerTestEnv) TaskRepository() task.Repository {
	return nil
}

func (e *handlerTestEnv) ResourceStore() resources.ResourceStore {
	return nil
}

type stubAgentExecutor struct {
	callCount int
	lastReq   toolenv.AgentRequest
}

func (s *stubAgentExecutor) ExecuteAgent(_ context.Context, req toolenv.AgentRequest) (*toolenv.AgentResult, error) {
	s.callCount++
	s.lastReq = req
	output := core.Output{"agent": req.AgentID}
	return &toolenv.AgentResult{
		ExecID: core.MustNewID(),
		Output: &output,
	}, nil
}
