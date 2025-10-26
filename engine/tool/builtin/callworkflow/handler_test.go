package callworkflow

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
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestHandlerExecutesWorkflow(t *testing.T) {
	t.Run("Should execute workflow and return result", func(t *testing.T) {
		ctx := attachConfig(t, nil)
		exec := &recordingWorkflowExecutor{}
		env := &stubEnvironment{workflowExec: exec}
		payload := map[string]any{
			"workflow_id": "user-onboarding",
			"input": map[string]any{
				"user_id": "123",
			},
		}
		output, err := newHandler(env)(ctx, payload)
		require.NoError(t, err)
		require.NotNil(t, output)
		exec.requireCalled(t)
		require.Equal(t, "user-onboarding", output["workflow_id"])
		require.Equal(t, true, output["success"])
		require.NotEmpty(t, output["workflow_exec_id"])
	})
}

func TestHandlerValidatesWorkflowID(t *testing.T) {
	t.Run("Should reject empty workflow id", func(t *testing.T) {
		ctx := attachConfig(t, nil)
		env := &stubEnvironment{workflowExec: &recordingWorkflowExecutor{}}
		_, err := newHandler(env)(ctx, map[string]any{})
		require.Error(t, err)
	})
}

func TestHandlerDisablesWhenConfigured(t *testing.T) {
	t.Run("Should block execution when tool disabled", func(t *testing.T) {
		cfg := config.DefaultNativeToolsConfig()
		cfg.CallWorkflow.Enabled = false
		ctx := attachConfig(t, &cfg)
		env := &stubEnvironment{workflowExec: &recordingWorkflowExecutor{}}
		_, err := newHandler(env)(ctx, map[string]any{"workflow_id": "noop"})
		require.Error(t, err)
	})
}

func TestHandlerUsesTimeoutOverride(t *testing.T) {
	t.Run("Should honor timeout override", func(t *testing.T) {
		ctx := attachConfig(t, nil)
		exec := &recordingWorkflowExecutor{}
		env := &stubEnvironment{workflowExec: exec}
		payload := map[string]any{
			"workflow_id": "batch",
			"timeout_ms":  1500,
		}
		_, err := newHandler(env)(ctx, payload)
		require.NoError(t, err)
		require.Len(t, exec.requests, 1)
		require.Equal(t, 1500*time.Millisecond, exec.requests[0].Timeout)
	})
}

func TestHandlerRejectsNegativeDefaultTimeout(t *testing.T) {
	t.Run("Should fail when default timeout is negative", func(t *testing.T) {
		cfg := config.DefaultNativeToolsConfig()
		cfg.CallWorkflow.DefaultTimeout = -1 * time.Second
		ctx := attachConfig(t, &cfg)
		env := &stubEnvironment{workflowExec: &recordingWorkflowExecutor{}}
		_, err := newHandler(env)(ctx, map[string]any{"workflow_id": "invalid-default"})
		require.Error(t, err)
		var cerr *core.Error
		require.True(t, errors.As(err, &cerr))
		require.Equal(t, builtin.CodeInvalidArgument, cerr.Code)
	})
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

type recordingWorkflowExecutor struct {
	requests []toolenv.WorkflowRequest
}

func (r *recordingWorkflowExecutor) ExecuteWorkflow(
	_ context.Context,
	req toolenv.WorkflowRequest,
) (*toolenv.WorkflowResult, error) {
	r.requests = append(r.requests, req)
	return &toolenv.WorkflowResult{
		WorkflowExecID: core.MustNewID(),
		Output:         &core.Output{"result": true},
		Status:         string(core.StatusSuccess),
	}, nil
}

func (r *recordingWorkflowExecutor) requireCalled(t *testing.T) {
	t.Helper()
	require.NotEmpty(t, r.requests)
}

type stubEnvironment struct {
	workflowExec toolenv.WorkflowExecutor
}

func (s *stubEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (s *stubEnvironment) TaskExecutor() toolenv.TaskExecutor         { return nil }
func (s *stubEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return s.workflowExec }
func (s *stubEnvironment) TaskRepository() task.Repository            { return nil }
func (s *stubEnvironment) ResourceStore() resources.ResourceStore     { return nil }
