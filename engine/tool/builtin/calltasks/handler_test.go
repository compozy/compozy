package calltasks

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestHandlerExecutesTasks(t *testing.T) {
	ctx := attachConfig(t, nil)
	exec := &recordingParallelExecutor{}
	env := &stubEnvironment{taskExec: exec}
	payload := map[string]any{
		"tasks": []any{
			map[string]any{"task_id": "normalize"},
			map[string]any{"task_id": "aggregate"},
		},
	}
	output, err := newHandler(env)(ctx, payload)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Len(t, exec.requests, 2)
	total, ok := output["total_count"].(int)
	require.True(t, ok)
	require.Equal(t, 2, total)
	failures, ok := output["failure_count"].(int)
	require.True(t, ok)
	require.Equal(t, 0, failures)
}

func TestHandlerValidatesTasksArray(t *testing.T) {
	ctx := attachConfig(t, nil)
	env := &stubEnvironment{taskExec: &recordingParallelExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{"tasks": []any{}})
	require.Error(t, err)
}

func TestHandlerDisabled(t *testing.T) {
	cfg := config.DefaultNativeToolsConfig()
	cfg.CallTasks.Enabled = false
	ctx := attachConfig(t, &cfg)
	env := &stubEnvironment{taskExec: &recordingParallelExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{"tasks": []any{map[string]any{"task_id": "noop"}}})
	require.Error(t, err)
}

type recordingParallelExecutor struct {
	requests []toolenv.TaskRequest
	count    atomic.Int64
}

func (r *recordingParallelExecutor) ExecuteTask(
	_ context.Context,
	req toolenv.TaskRequest,
) (*toolenv.TaskResult, error) {
	r.count.Add(1)
	r.requests = append(r.requests, req)
	return &toolenv.TaskResult{ExecID: core.MustNewID(), Output: &core.Output{"ok": true}}, nil
}

type stubEnvironment struct {
	taskExec toolenv.TaskExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (s *stubEnvironment) TaskExecutor() toolenv.TaskExecutor         { return s.taskExec }
func (s *stubEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return nil }
func (s *stubEnvironment) TaskRepository() task.Repository            { return nil }
func (s *stubEnvironment) ResourceStore() resources.ResourceStore     { return nil }

func attachConfig(t *testing.T, override *config.NativeToolsConfig) context.Context {
	manager := config.NewManager(t.Context(), config.NewService())
	cfg, err := manager.Load(t.Context(), config.NewDefaultProvider())
	require.NoError(t, err)
	if override != nil {
		cfg.Runtime.NativeTools = *override
	}
	return config.ContextWithManager(t.Context(), manager)
}
