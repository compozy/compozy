package tool

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sync/atomic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	_ "github.com/compozy/compozy/engine/tool/builtin/imports"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestCallTasksIntegration(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	project := "demo"
	ctx = core.WithProjectName(ctx, project)
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.Enabled = true
	cfg.Runtime.NativeTools.CallTask.Enabled = true
	cfg.Runtime.NativeTools.CallTasks.Enabled = true
	cfg.Runtime.NativeTools.CallTasks.MaxConcurrent = 4
	ctx = config.ContextWithManager(ctx, manager)
	executor := &parallelTaskExecutor{}
	env := &taskEnvironment{taskExec: executor, store: resources.NewMemoryResourceStore()}
	payload := map[string]any{
		"tasks": []any{
			map[string]any{"task_id": "one"},
			map[string]any{"task_id": "two"},
		},
	}
	callArgs, err := json.Marshal(payload)
	require.NoError(t, err)
	script := newScriptedLLM([]*llmadapter.LLMResponse{
		{
			Content: "",
			ToolCalls: []llmadapter.ToolCall{
				{ID: "call-tasks", Name: "cp__call_tasks", Arguments: callArgs},
			},
		},
		{Content: "Parallel tasks complete"},
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
	output, err := service.GenerateContent(ctx, agentCfg, nil, "", "parallel tasks", nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Parallel tasks complete", output.Prop("response"))
	require.Equal(t, int64(2), executor.count.Load())
	requests := script.Requests()
	require.Len(t, requests, 2)
	result, ok := findToolResult(requests[1].Messages, "cp__call_tasks")
	require.True(t, ok)
	var payloadOut map[string]any
	require.NoError(t, json.Unmarshal(result.JSONContent, &payloadOut))
	assert.Equal(t, float64(2), payloadOut["total_count"])
}

type parallelTaskExecutor struct {
	count atomic.Int64
}

func (p *parallelTaskExecutor) ExecuteTask(_ context.Context, req toolenv.TaskRequest) (*toolenv.TaskResult, error) {
	p.count.Add(1)
	return &toolenv.TaskResult{
		ExecID: core.MustNewID(),
		Output: &core.Output{"ok": true},
	}, nil
}
