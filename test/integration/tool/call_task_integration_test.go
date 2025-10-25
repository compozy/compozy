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

func TestCallTaskIntegration(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	project := "demo"
	ctx = core.WithProjectName(ctx, project)
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.Enabled = true
	cfg.Runtime.NativeTools.CallTask.Enabled = true
	ctx = config.ContextWithManager(ctx, manager)
	executor := &recordingTaskExecutor{}
	env := &taskEnvironment{taskExec: executor, store: resources.NewMemoryResourceStore()}
	payload := map[string]any{
		"task_id": "transform.dataset",
	}
	callArgs, err := json.Marshal(payload)
	require.NoError(t, err)
	script := newScriptedLLM([]*llmadapter.LLMResponse{
		{
			Content: "",
			ToolCalls: []llmadapter.ToolCall{
				{ID: "call-task", Name: "cp__call_task", Arguments: callArgs},
			},
		},
		{Content: "Task execution complete"},
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
	output, err := service.GenerateContent(ctx, agentCfg, nil, "", "invoke task", nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Task execution complete", output.Prop("response"))
	require.Len(t, executor.requests, 1)
	assert.Equal(t, "transform.dataset", executor.requests[0].TaskID)
	requests := script.Requests()
	require.Len(t, requests, 2)
	result, ok := findToolResult(requests[1].Messages, "cp__call_task")
	require.True(t, ok)
	require.NotEmpty(t, result.JSONContent)
}

type taskEnvironment struct {
	taskExec toolenv.TaskExecutor
	store    resources.ResourceStore
}

func (e *taskEnvironment) AgentExecutor() toolenv.AgentExecutor       { return nil }
func (e *taskEnvironment) TaskExecutor() toolenv.TaskExecutor         { return e.taskExec }
func (e *taskEnvironment) WorkflowExecutor() toolenv.WorkflowExecutor { return nil }
func (e *taskEnvironment) TaskRepository() task.Repository            { return nil }
func (e *taskEnvironment) ResourceStore() resources.ResourceStore     { return e.store }

type recordingTaskExecutor struct {
	requests []toolenv.TaskRequest
}

func (r *recordingTaskExecutor) ExecuteTask(_ context.Context, req toolenv.TaskRequest) (*toolenv.TaskResult, error) {
	r.requests = append(r.requests, req)
	return &toolenv.TaskResult{
		ExecID: core.MustNewID(),
		Output: &core.Output{"result": true},
	}, nil
}
