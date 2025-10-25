package callworkflows

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

func TestHandlerExecutesWorkflows(t *testing.T) {
	ctx := attachConfig(t, nil)
	exec := &recordingParallelExecutor{}
	env := &stubEnvironment{workflowExec: exec}
	payload := map[string]any{
		"workflows": []any{
			map[string]any{"workflow_id": "onboard"},
			map[string]any{"workflow_id": "provision"},
		},
	}
	output, err := newHandler(env)(ctx, payload)
	require.NoError(t, err)
	require.NotNil(t, output)
	total, ok := output["total_count"].(int)
	require.True(t, ok)
	require.Equal(t, 2, total)
	require.Equal(t, int64(2), exec.count.Load())
}

func TestHandlerValidatesWorkflowsArray(t *testing.T) {
	ctx := attachConfig(t, nil)
	env := &stubEnvironment{workflowExec: &recordingParallelExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{"workflows": []any{}})
	require.Error(t, err)
}

func TestHandlerDisabled(t *testing.T) {
	cfg := config.DefaultNativeToolsConfig()
	cfg.CallWorkflows.Enabled = false
	ctx := attachConfig(t, &cfg)
	env := &stubEnvironment{workflowExec: &recordingParallelExecutor{}}
	_, err := newHandler(env)(ctx, map[string]any{"workflows": []any{map[string]any{"workflow_id": "noop"}}})
	require.Error(t, err)
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

type recordingParallelExecutor struct {
	count atomic.Int64
}

func (r *recordingParallelExecutor) ExecuteWorkflow(
	_ context.Context,
	_ toolenv.WorkflowRequest,
) (*toolenv.WorkflowResult, error) {
	r.count.Add(1)
	return &toolenv.WorkflowResult{
		WorkflowExecID: core.MustNewID(),
		Status:         string(core.StatusSuccess),
		Output:         &core.Output{"ok": true},
	}, nil
}

type stubEnvironment struct {
	workflowExec toolenv.WorkflowExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (s *stubEnvironment) TaskExecutor() toolenv.TaskExecutor         { return nil }
func (s *stubEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return s.workflowExec }
func (s *stubEnvironment) TaskRepository() task.Repository            { return nil }
func (s *stubEnvironment) ResourceStore() resources.ResourceStore     { return nil }
