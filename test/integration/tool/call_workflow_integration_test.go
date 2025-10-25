package tool

import (
	"context"
	"encoding/json"
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
	_ "github.com/compozy/compozy/engine/tool/builtin/imports"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestCallWorkflowIntegration(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	project := "demo"
	ctx = core.WithProjectName(ctx, project)
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.Enabled = true
	cfg.Runtime.NativeTools.CallWorkflow.Enabled = true
	ctx = config.ContextWithManager(ctx, manager)
	executor := &recordingWorkflowExecutor{}
	env := &workflowEnvironment{workflowExec: executor, store: resources.NewMemoryResourceStore()}
	payload := map[string]any{
		"workflow_id": "user.onboarding",
	}
	callArgs, err := json.Marshal(payload)
	require.NoError(t, err)
	script := newScriptedLLM([]*llmadapter.LLMResponse{
		{
			Content: "",
			ToolCalls: []llmadapter.ToolCall{
				{ID: "call-workflow", Name: "cp__call_workflow", Arguments: callArgs},
			},
		},
		{Content: "Workflow complete"},
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
	output, err := service.GenerateContent(ctx, agentCfg, nil, "", "call workflow", nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Workflow complete", output.Prop("response"))
	require.Len(t, executor.requests, 1)
	assert.Equal(t, "user.onboarding", executor.requests[0].WorkflowID)
	requests := script.Requests()
	require.Len(t, requests, 2)
	result, ok := findToolResult(requests[1].Messages, "cp__call_workflow")
	require.True(t, ok)
	require.NotEmpty(t, result.JSONContent)
}

type workflowEnvironment struct {
	workflowExec toolenv.WorkflowExecutor
	store        resources.ResourceStore
}

func (e *workflowEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (e *workflowEnvironment) TaskExecutor() toolenv.TaskExecutor         { return nil }
func (e *workflowEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return e.workflowExec }
func (e *workflowEnvironment) TaskRepository() task.Repository            { return nil }
func (e *workflowEnvironment) ResourceStore() resources.ResourceStore     { return e.store }

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
		Status:         string(core.StatusSuccess),
		Output:         &core.Output{"ok": true},
	}, nil
}
