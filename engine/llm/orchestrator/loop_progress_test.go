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

func TestConversationLoop_NoProgressDetection(t *testing.T) {
	cfg := &settings{maxToolIterations: 5, enableProgressTracking: true, noProgressThreshold: 2}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "t"}}}}
	exec := &noOpExec{results: []llmadapter.ToolResult{{ID: "1", Name: "t", Content: `{"ok":true}`}}}
	h := passHandler{}
	loop := newConversationLoop(cfg, exec, h, inv, nil)
	llmReq := &llmadapter.LLMRequest{}
	state := newLoopState(cfg, nil, &agent.ActionConfig{ID: "a"})
	req := Request{Agent: &agent.Config{ID: "ag"}, Action: &agent.ActionConfig{ID: "a"}}
	_, _, err := loop.Run(context.Background(), nil, llmReq, req, state)
	require.Error(t, err)
	require.Equal(t, 3, exec.execCalls)
	require.Equal(t, 3, exec.budgetCalls)
}
