package agentexec_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	agentexec "github.com/compozy/compozy/engine/agent/exec"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/stretchr/testify/require"
)

type stubDirectExecutor struct {
	output       *core.Output
	execID       core.ID
	err          error
	asyncExecID  core.ID
	asyncErr     error
	lastMetadata tkrouter.ExecMetadata
	lastTimeout  time.Duration
}

func (s *stubDirectExecutor) ExecuteSync(
	_ context.Context,
	cfg *task.Config,
	meta *tkrouter.ExecMetadata,
	timeout time.Duration,
) (*core.Output, core.ID, error) {
	s.lastTimeout = timeout
	if meta != nil {
		s.lastMetadata = *meta
	}
	if cfg != nil && cfg.With != nil {
		if cloned, err := core.DeepCopy(cfg.With); err == nil {
			*cfg.With = *cloned
		}
	}
	return s.output, s.execID, s.err
}

func (s *stubDirectExecutor) ExecuteAsync(context.Context, *task.Config, *tkrouter.ExecMetadata) (core.ID, error) {
	return s.asyncExecID, s.asyncErr
}

func installExecutorStub(state *appstate.State, exec tkrouter.DirectExecutor) func() {
	tkrouter.SetDirectExecutorFactory(state, func(*appstate.State, task.Repository) (tkrouter.DirectExecutor, error) {
		return exec, nil
	})
	return func() { tkrouter.SetDirectExecutorFactory(state, nil) }
}

func storeAgent(t *testing.T, store resources.ResourceStore, state *appstate.State, cfg *agent.Config) {
	t.Helper()
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(context.Background(), resources.ResourceKey{
		Project: state.ProjectConfig.Name,
		Type:    resources.ResourceAgent,
		ID:      cfg.ID,
	}, m)
	require.NoError(t, err)
}

func setupRunner(
	t *testing.T,
	stub *stubDirectExecutor,
	cfg *agent.Config,
) (*agentexec.Runner, func()) {
	state := routertest.NewTestAppState(t)
	store := routertest.NewResourceStore(state)
	repo := routertest.NewStubTaskRepo()
	cleanup := installExecutorStub(state, stub)
	if cfg != nil {
		storeAgent(t, store, state, cfg)
	}
	runner := agentexec.NewRunner(state, repo, store)
	return runner, func() { cleanup() }
}

func TestRunnerExecute(t *testing.T) {
	t.Run("ShouldRunAgent", func(t *testing.T) {
		execID := core.MustNewID()
		output := core.Output{"value": "ok"}
		stub := &stubDirectExecutor{output: &output, execID: execID}
		runner, cleanup := setupRunner(t, stub, &agent.Config{
			ID: "agent-one",
			Actions: []*agent.ActionConfig{
				{ID: "summary"},
			},
		})
		defer cleanup()
		req := agentexec.ExecuteRequest{
			AgentID: "agent-one",
			Action:  "summary",
			With:    core.Input{"topic": "status"},
			Timeout: 42 * time.Second,
		}
		result, err := runner.Execute(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, execID, result.ExecID)
		require.Equal(t, stub.output, result.Output)
		require.Equal(t, 42*time.Second, stub.lastTimeout)
		require.Equal(t, "summary", stub.lastMetadata.ActionID)
	})
	t.Run("ShouldReturnNotFound", func(t *testing.T) {
		runner, cleanup := setupRunner(t, &stubDirectExecutor{}, nil)
		defer cleanup()
		req := agentexec.ExecuteRequest{
			AgentID: "missing-agent",
			Action:  "summary",
			Timeout: 5 * time.Second,
		}
		result, err := runner.Execute(context.Background(), req)
		require.Nil(t, result)
		require.Error(t, err)
		require.True(t, errors.Is(err, agentuc.ErrNotFound))
	})
	t.Run("ShouldValidateAction", func(t *testing.T) {
		stub := &stubDirectExecutor{}
		runner, cleanup := setupRunner(t, stub, &agent.Config{
			ID:      "agent-one",
			Actions: []*agent.ActionConfig{{ID: "allowed"}},
		})
		defer cleanup()
		req := agentexec.ExecuteRequest{
			AgentID: "agent-one",
			Action:  "missing",
			Timeout: 5 * time.Second,
		}
		result, err := runner.Execute(context.Background(), req)
		require.Nil(t, result)
		require.Error(t, err)
		require.True(t, errors.Is(err, agentexec.ErrUnknownAction))
	})
	t.Run("ShouldPropagateTimeout", func(t *testing.T) {
		execID := core.MustNewID()
		stub := &stubDirectExecutor{execID: execID, err: context.DeadlineExceeded}
		runner, cleanup := setupRunner(t, stub, &agent.Config{ID: "agent-one"})
		defer cleanup()
		req := agentexec.ExecuteRequest{
			AgentID: "agent-one",
			Prompt:  "run",
		}
		result, err := runner.Execute(context.Background(), req)
		require.NotNil(t, result)
		require.Equal(t, execID, result.ExecID)
		require.Nil(t, result.Output)
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
	t.Run("ShouldWrapExecutorError", func(t *testing.T) {
		stubErr := errors.New("boom")
		stub := &stubDirectExecutor{execID: core.MustNewID(), err: stubErr}
		runner, cleanup := setupRunner(t, stub, &agent.Config{ID: "agent-one"})
		defer cleanup()
		req := agentexec.ExecuteRequest{
			AgentID: "agent-one",
			Prompt:  "run",
			Timeout: time.Second,
		}
		result, err := runner.Execute(context.Background(), req)
		require.NotNil(t, result)
		require.Equal(t, stub.execID, result.ExecID)
		require.Error(t, err)
		require.True(t, errors.Is(err, stubErr))
		require.Contains(t, err.Error(), "agent execution failed")
	})
}

func TestRunnerPrepare(t *testing.T) {
	t.Run("ShouldDefaultTimeoutAndMetadata", func(t *testing.T) {
		stub := &stubDirectExecutor{}
		runner, cleanup := setupRunner(t, stub, &agent.Config{ID: "agent-one"})
		defer cleanup()
		prepared, err := runner.Prepare(context.Background(), agentexec.ExecuteRequest{
			AgentID: "agent-one",
			Prompt:  "hello",
		})
		require.NoError(t, err)
		require.NotNil(t, prepared)
		require.Equal(t, agentexec.DefaultTimeout, prepared.Timeout)
		require.Equal(t, "agent:agent-one", prepared.Config.ID)
		require.Equal(t, "__prompt__", prepared.Metadata.ActionID)
	})
}
