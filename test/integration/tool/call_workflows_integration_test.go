package tool

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

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

func TestCallWorkflowsIntegration(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	project := "demo"
	ctx = core.WithProjectName(ctx, project)
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Runtime.NativeTools.Enabled = true
	cfg.Runtime.NativeTools.CallWorkflow.Enabled = true
	cfg.Runtime.NativeTools.CallWorkflows.Enabled = true
	cfg.Runtime.NativeTools.CallWorkflows.MaxConcurrent = 3
	ctx = config.ContextWithManager(ctx, manager)
	executor := &parallelWorkflowExecutor{}
	env := &workflowEnvironment{workflowExec: executor, store: resources.NewMemoryResourceStore()}
	payload := map[string]any{
		"workflows": []any{
			map[string]any{"workflow_id": "process.us"},
			map[string]any{"workflow_id": "process.eu"},
		},
	}
	callArgs, err := json.Marshal(payload)
	require.NoError(t, err)
	script := newScriptedLLM([]*llmadapter.LLMResponse{
		{
			Content: "",
			ToolCalls: []llmadapter.ToolCall{
				{ID: "call-workflows", Name: "cp__call_workflows", Arguments: callArgs},
			},
		},
		{Content: "Parallel workflows complete"},
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
	output, err := service.GenerateContent(ctx, agentCfg, nil, "", "parallel workflows", nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Parallel workflows complete", output.Prop("response"))
	require.Equal(t, int64(2), executor.count.Load())
	requests := script.Requests()
	require.Len(t, requests, 2)
	result, ok := findToolResult(requests[1].Messages, "cp__call_workflows")
	require.True(t, ok)
	var payloadOut map[string]any
	require.NoError(t, json.Unmarshal(result.JSONContent, &payloadOut))
	assert.Equal(t, float64(2), payloadOut["total_count"])
}

type parallelWorkflowExecutor struct {
	count atomic.Int64
}

func (p *parallelWorkflowExecutor) ExecuteWorkflow(
	_ context.Context,
	req toolenv.WorkflowRequest,
) (*toolenv.WorkflowResult, error) {
	p.count.Add(1)
	return &toolenv.WorkflowResult{
		WorkflowExecID: core.MustNewID(),
		Status:         string(core.StatusSuccess),
		Output:         &core.Output{"ok": true},
	}, nil
}
