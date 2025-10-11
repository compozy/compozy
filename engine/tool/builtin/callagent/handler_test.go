package callagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinitionRegisters(t *testing.T) {
	env := &stubEnvironment{}
	def := Definition(env)
	assert.Equal(t, toolID, def.ID)

	defs := native.Definitions(env)
	found := false
	for _, d := range defs {
		if d.ID == toolID {
			found = true
		}
	}
	assert.True(t, found, "cp__call_agent should be discoverable via native definitions")
}

func TestHandlerRequiresExecutor(t *testing.T) {
	ctx := context.Background()
	env := &stubEnvironment{}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"agent_id": "agent.alpha",
		"prompt":   "hello world",
	})
	require.Error(t, err)
	var cerr *core.Error
	require.True(t, errors.As(err, &cerr))
	assert.Equal(t, builtin.CodeInternal, cerr.Code)
}

func TestHandlerValidatesAgentID(t *testing.T) {
	ctx := attachConfig(context.Background(), nil)
	exec := &stubAgentExecutor{}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"prompt": "perform action",
	})
	require.Error(t, err)
	var cerr *core.Error
	require.True(t, errors.As(err, &cerr))
	assert.Equal(t, builtin.CodeInvalidArgument, cerr.Code)
}

func TestHandlerRequiresActionOrPrompt(t *testing.T) {
	ctx := attachConfig(context.Background(), nil)
	exec := &stubAgentExecutor{}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"agent_id": "agent.beta",
	})
	require.Error(t, err)
	var cerr *core.Error
	require.True(t, errors.As(err, &cerr))
	assert.Equal(t, builtin.CodeInvalidArgument, cerr.Code)
}

func TestHandlerRejectsNegativeTimeout(t *testing.T) {
	ctx := attachConfig(context.Background(), nil)
	exec := &stubAgentExecutor{}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"agent_id":   "agent.gamma",
		"prompt":     "run it",
		"timeout_ms": -10,
	})
	require.Error(t, err)
	var cerr *core.Error
	require.True(t, errors.As(err, &cerr))
	assert.Equal(t, builtin.CodeInvalidArgument, cerr.Code)
}

func TestHandlerExecutesAgentWithDefaults(t *testing.T) {
	ctx := attachConfig(context.Background(), nil)
	output := core.Output{"result": "ok"}
	exec := &stubAgentExecutor{
		result: &toolenv.AgentResult{
			ExecID: core.ID("exec-123"),
			Output: &output,
		},
	}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	payload := map[string]any{
		"agent_id": "agent.delta",
		"prompt":   "summarize",
		"with": map[string]any{
			"topic": "testing",
		},
	}
	result, err := handler(ctx, payload)
	require.NoError(t, err)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "agent.delta", result["agent_id"])
	assert.Equal(t, "exec-123", result["exec_id"])
	response, ok := result["response"].(core.Output)
	require.True(t, ok)
	assert.Equal(t, "ok", response["result"])
	assert.Equal(t, "agent.delta", exec.lastReq.AgentID)
	assert.Equal(t, "summarize", exec.lastReq.Prompt)
	assert.Equal(t, 60*time.Second, exec.lastReq.Timeout)
	assert.Equal(t, core.Input{"topic": "testing"}, exec.lastReq.With)
	assert.Equal(t, 1, exec.callCount)
}

func TestHandlerRespectsTimeoutOverride(t *testing.T) {
	cfg := config.DefaultNativeToolsConfig()
	cfg.CallAgent.DefaultTimeout = 10 * time.Second
	ctx := attachConfig(context.Background(), &cfg)
	exec := &stubAgentExecutor{
		result: &toolenv.AgentResult{
			ExecID: core.ID("exec-555"),
		},
	}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"agent_id":   "agent.theta",
		"action_id":  "compute",
		"timeout_ms": 2500,
	})
	require.NoError(t, err)
	require.Equal(t, time.Duration(2500)*time.Millisecond, exec.lastReq.Timeout)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type stubAgentExecutor struct {
	result    *toolenv.AgentResult
	err       error
	lastReq   toolenv.AgentRequest
	callCount int
}

func (s *stubAgentExecutor) ExecuteAgent(_ context.Context, req toolenv.AgentRequest) (*toolenv.AgentResult, error) {
	s.lastReq = req
	s.callCount++
	return s.result, s.err
}

type stubEnvironment struct {
	executor toolenv.AgentExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor {
	return s.executor
}

func (s *stubEnvironment) TaskRepository() task.Repository {
	return nil
}

func (s *stubEnvironment) ResourceStore() resources.ResourceStore {
	return nil
}

func attachConfig(ctx context.Context, cfgOverride *config.NativeToolsConfig) context.Context {
	manager := config.NewManager(config.NewService())
	cfg, err := manager.Load(context.Background(), config.NewDefaultProvider())
	if err != nil {
		panic(err)
	}
	if cfgOverride != nil {
		cfg.Runtime.NativeTools = *cfgOverride
	}
	return config.ContextWithManager(ctx, manager)
}
