package callagents

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallAgentsDefinition(t *testing.T) {
	t.Parallel()

	t.Run("Should register definition", func(t *testing.T) {
		t.Parallel()
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
		assert.True(t, found, "cp__call_agents should be discoverable via native definitions")
	})

	t.Run("Should require executor", func(t *testing.T) {
		t.Parallel()
		ctx := attachConfig(t, nil)
		env := &stubEnvironment{}
		handler := Definition(env).Handler
		_, err := handler(ctx, map[string]any{
			"agents": []any{
				map[string]any{"agent_id": "alpha", "prompt": "hello"},
			},
		})
		require.Error(t, err)
		var cerr *core.Error
		require.True(t, errors.As(err, &cerr))
		assert.Equal(t, builtin.CodeInternal, cerr.Code)
	})

	t.Run("Should reject when disabled", func(t *testing.T) {
		t.Parallel()
		override := func(cfg *config.Config) {
			cfg.Runtime.NativeTools.CallAgents.Enabled = false
		}
		ctx := attachConfig(t, override)
		env := &stubEnvironment{executor: &stubAgentExecutor{}}
		handler := Definition(env).Handler
		_, err := handler(ctx, map[string]any{
			"agents": []any{
				map[string]any{"agent_id": "alpha", "prompt": "hello"},
			},
		})
		require.Error(t, err)
		var cerr *core.Error
		require.True(t, errors.As(err, &cerr))
		assert.Equal(t, builtin.CodePermissionDenied, cerr.Code)
	})
}

func TestHandlerValidatesAgents(t *testing.T) {
	t.Parallel()
	ctx := attachConfig(t, nil)
	env := &stubEnvironment{executor: &stubAgentExecutor{}}
	handler := Definition(env).Handler
	_, err := handler(ctx, map[string]any{
		"agents": []any{
			map[string]any{"prompt": "missing id"},
		},
	})
	require.Error(t, err)
	var cerr *core.Error
	require.True(t, errors.As(err, &cerr))
	assert.Equal(t, builtin.CodeInvalidArgument, cerr.Code)
}

func TestHandlerEnforcesAgentCountLimit(t *testing.T) {
	t.Parallel()
	override := func(cfg *config.Config) {
		cfg.Runtime.NativeTools.CallAgents.MaxConcurrent = 1
	}
	ctx := attachConfig(t, override)
	exec := &stubAgentExecutor{
		responses: map[string]*toolenv.AgentResult{
			"alpha": {ExecID: core.MustNewID()},
			"beta":  {ExecID: core.MustNewID()},
		},
	}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	result, err := handler(ctx, map[string]any{
		"agents": []any{
			map[string]any{"agent_id": "alpha", "prompt": "first"},
			map[string]any{"agent_id": "beta", "prompt": "second"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result["total_count"])
	assert.Equal(t, 2, result["success_count"])
}

func TestHandlerExecutesAgentsAndAggregatesResults(t *testing.T) {
	t.Parallel()
	ctx := attachConfig(t, nil)
	exec := &stubAgentExecutor{
		responses: map[string]*toolenv.AgentResult{
			"alpha": {
				ExecID: core.MustNewID(),
				Output: &core.Output{"value": 1},
			},
			"gamma": {
				ExecID: core.MustNewID(),
			},
		},
		errors: map[string]error{
			"beta":  builtin.InvalidArgument(errors.New("bad input"), map[string]any{"field": "with"}),
			"gamma": context.DeadlineExceeded,
		},
	}
	env := &stubEnvironment{executor: exec}
	handler := Definition(env).Handler
	payload := map[string]any{
		"agents": []any{
			map[string]any{"agent_id": "alpha", "prompt": "first"},
			map[string]any{"agent_id": "beta", "prompt": "second"},
			map[string]any{"agent_id": "gamma", "prompt": "third", "timeout_ms": 2500},
		},
	}
	result, err := handler(ctx, payload)
	require.NoError(t, err)
	results, ok := result["results"].([]AgentExecutionResult)
	require.True(t, ok)
	require.Len(t, results, 3)
	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success)
	require.NotNil(t, results[1].Error)
	assert.Equal(t, builtin.CodeInvalidArgument, results[1].Error.Code)
	assert.False(t, results[2].Success)
	require.NotNil(t, results[2].Error)
	assert.Equal(t, builtin.CodeDeadlineExceeded, results[2].Error.Code)
	assert.Equal(t, 3, result["total_count"])
	assert.Equal(t, 1, result["success_count"])
	assert.Equal(t, 2, result["failure_count"])
	totalDuration, ok := result["total_duration_ms"].(int64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, totalDuration, int64(0))
	require.Len(t, exec.requests, 3)
	alphaReq, ok := findRequest(exec.requests, "alpha")
	require.True(t, ok)
	defaultCfg := config.DefaultNativeToolsConfig()
	assert.Equal(t, defaultCfg.CallAgents.DefaultTimeout, alphaReq.Timeout)
	gammaReq, ok := findRequest(exec.requests, "gamma")
	require.True(t, ok)
	assert.Equal(t, 2500*time.Millisecond, gammaReq.Timeout)
}

// --------------------------------------------------------------------------- //
// Test helpers
// --------------------------------------------------------------------------- //

type stubAgentExecutor struct {
	responses map[string]*toolenv.AgentResult
	errors    map[string]error
	requests  []toolenv.AgentRequest
	mu        sync.Mutex
}

func (s *stubAgentExecutor) ExecuteAgent(_ context.Context, req toolenv.AgentRequest) (*toolenv.AgentResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	resp := s.responses[req.AgentID]
	err := s.errors[req.AgentID]
	s.mu.Unlock()
	return resp, err
}

func findRequest(requests []toolenv.AgentRequest, agentID string) (toolenv.AgentRequest, bool) {
	for _, req := range requests {
		if req.AgentID == agentID {
			return req, true
		}
	}
	return toolenv.AgentRequest{}, false
}

func attachConfig(t *testing.T, mutate func(*config.Config)) context.Context {
	t.Helper()
	ctx := t.Context()
	log := logger.NewForTests()
	ctx = logger.ContextWithLogger(ctx, log)
	manager := config.NewManager(ctx, config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	if mutate != nil {
		cfg := manager.Get()
		mutate(cfg)
	}
	return config.ContextWithManager(ctx, manager)
}
