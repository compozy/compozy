package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

type staticInvoker struct {
	resp *llmadapter.LLMResponse
	err  error
}

func (s *staticInvoker) Invoke(
	context.Context,
	llmadapter.LLMClient,
	*llmadapter.LLMRequest,
	Request,
) (*llmadapter.LLMResponse, error) {
	return s.resp, s.err
}

type noOpExec struct {
	results     []llmadapter.ToolResult
	err         error
	execCalls   int
	budgetCalls int
}

func (n *noOpExec) Execute(context.Context, []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	n.execCalls++
	return n.results, n.err
}
func (n *noOpExec) UpdateBudgets(context.Context, []llmadapter.ToolResult, *loopState) error {
	n.budgetCalls++
	return nil
}

type passHandler struct{}

func (passHandler) HandleNoToolCalls(
	context.Context,
	*llmadapter.LLMResponse,
	Request,
	*llmadapter.LLMRequest,
	*loopState,
) (*core.Output, bool, error) {
	return nil, false, nil
}

type recordingMemoryManager struct {
	storeCalls int
}

func (r *recordingMemoryManager) Prepare(_ context.Context, _ Request) *MemoryContext {
	return &MemoryContext{}
}

func (r *recordingMemoryManager) Inject(
	_ context.Context,
	base []llmadapter.Message,
	_ *MemoryContext,
) []llmadapter.Message {
	return base
}

func (r *recordingMemoryManager) StoreAsync(
	_ context.Context,
	_ *MemoryContext,
	_ *llmadapter.LLMResponse,
	_ []llmadapter.Message,
	_ Request,
) {
	r.storeCalls++
}

func TestConversationLoop_NoProgressDetection(t *testing.T) {
	cfg := &settings{maxToolIterations: 5, enableProgressTracking: true, noProgressThreshold: 2}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "t"}}}}
	exec := &noOpExec{results: []llmadapter.ToolResult{{ID: "1", Name: "t", Content: `{"ok":true}`}}}
	h := passHandler{}
	memory := &recordingMemoryManager{}
	loop := newConversationLoop(cfg, exec, h, inv, memory)
	llmReq := &llmadapter.LLMRequest{}
	state := newLoopState(cfg, &MemoryContext{}, &agent.ActionConfig{ID: "a"})
	req := Request{Agent: &agent.Config{ID: "ag"}, Action: &agent.ActionConfig{ID: "a"}}
	_, _, err := loop.Run(context.Background(), nil, llmReq, req, state)
	require.Error(t, err)
	require.Equal(t, 3, exec.execCalls)
	require.Equal(t, 3, exec.budgetCalls)
	require.Equal(t, 0, memory.storeCalls)
}

func TestConversationLoop_FinalizeStoresMemory(t *testing.T) {
	memory := &recordingMemoryManager{}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{Content: "done"}}
	exec := &noOpExec{}
	h := passHandler{}
	loop := newConversationLoop(&settings{maxToolIterations: 1}, exec, h, inv, memory)
	llmReq := &llmadapter.LLMRequest{Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hi"}}}
	memCtx := &MemoryContext{}
	state := newLoopState(&settings{}, memCtx, &agent.ActionConfig{ID: "action"})
	req := Request{Agent: &agent.Config{ID: "ag"}, Action: &agent.ActionConfig{ID: "action"}}
	output, _, err := loop.Run(context.Background(), nil, llmReq, req, state)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, 1, memory.storeCalls)
}
