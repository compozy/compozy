package calltask

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestHandlerExecutesTask(t *testing.T) {
	ctx := attachConfig(t, nil)
	stub := &recordingTaskExecutor{}
	env := &stubEnvironment{taskExec: stub}
	payload := map[string]any{
		"task_id": "data-clean",
		"with": map[string]any{
			"dataset": "users",
		},
	}
	output, err := newHandler(env)(ctx, payload)
	require.NoError(t, err)
	require.NotNil(t, output)
	stub.requireCalled(t)
	require.Equal(t, "data-clean", output["task_id"])
	require.Equal(t, true, output["success"])
	require.NotEmpty(t, output["exec_id"])
}

func TestHandlerValidatesTaskID(t *testing.T) {
	ctx := attachConfig(t, nil)
	env := &stubEnvironment{taskExec: &recordingTaskExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{})
	require.Error(t, err)
}

func TestHandlerDisablesWhenConfigured(t *testing.T) {
	cfg := config.DefaultNativeToolsConfig()
	cfg.CallTask.Enabled = false
	ctx := attachConfig(t, &cfg)
	env := &stubEnvironment{taskExec: &recordingTaskExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{"task_id": "noop"})
	require.Error(t, err)
}

func TestHandlerUsesTimeoutOverride(t *testing.T) {
	ctx := attachConfig(t, nil)
	stub := &recordingTaskExecutor{}
	env := &stubEnvironment{taskExec: stub}
	payload := map[string]any{
		"task_id":    "batch",
		"timeout_ms": 1200,
	}
	_, err := newHandler(env)(ctx, payload)
	require.NoError(t, err)
	require.Len(t, stub.requests, 1)
	require.Equal(t, 1200*time.Millisecond, stub.requests[0].Timeout)
}

func attachConfig(t *testing.T, override *config.NativeToolsConfig) context.Context {
	manager := config.NewManager(t.Context(), config.NewService())
	cfg, err := manager.Load(t.Context(), config.NewDefaultProvider())
	require.NoError(t, err)
	if override != nil {
		cfg.Runtime.NativeTools = *override
	}
	return config.ContextWithManager(t.Context(), manager)
}

type recordingTaskExecutor struct {
	requests []toolenv.TaskRequest
}

func (r *recordingTaskExecutor) ExecuteTask(_ context.Context, req toolenv.TaskRequest) (*toolenv.TaskResult, error) {
	r.requests = append(r.requests, req)
	return &toolenv.TaskResult{ExecID: core.MustNewID(), Output: &core.Output{"result": "ok"}}, nil
}

func (r *recordingTaskExecutor) requireCalled(t *testing.T) {
	t.Helper()
	require.NotEmpty(t, r.requests)
}

type stubEnvironment struct {
	taskExec toolenv.TaskExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (s *stubEnvironment) TaskExecutor() toolenv.TaskExecutor         { return s.taskExec }
func (s *stubEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return nil }
func (s *stubEnvironment) TaskRepository() task.Repository            { return nil }
func (s *stubEnvironment) ResourceStore() resources.ResourceStore     { return nil }
