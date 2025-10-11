package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

type noopExecutor struct{}

func (noopExecutor) Execute(context.Context, []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	return nil, nil
}

func (noopExecutor) UpdateBudgets(context.Context, []llmadapter.ToolResult, *loopState) error {
	return nil
}

type noopResponseHandler struct{}

func (noopResponseHandler) HandleNoToolCalls(
	context.Context,
	*llmadapter.LLMResponse,
	Request,
	*llmadapter.LLMRequest,
	*loopState,
) (*core.Output, bool, error) {
	return nil, false, nil
}

type completionHandlerStub struct {
	output *core.Output
	cont   bool
	err    error
	calls  int
}

func (c *completionHandlerStub) HandleNoToolCalls(
	context.Context,
	*llmadapter.LLMResponse,
	Request,
	*llmadapter.LLMRequest,
	*loopState,
) (*core.Output, bool, error) {
	c.calls++
	return c.output, c.cont, c.err
}

type memoryManagerStub struct {
	storeCalls      int
	ctxData         *MemoryContext
	response        *llmadapter.LLMResponse
	messages        []llmadapter.Message
	request         Request
	failureCalls    int
	failureCtx      *MemoryContext
	failureRequest  Request
	failureEpisodes []FailureEpisode
	compactCalls    int
}

func (m *memoryManagerStub) Prepare(context.Context, Request) *MemoryContext {
	return nil
}

func (m *memoryManagerStub) Inject(
	_ context.Context,
	base []llmadapter.Message,
	_ *MemoryContext,
) []llmadapter.Message {
	return base
}

//nolint:gocritic // Test stub mirrors production interface signature.
func (m *memoryManagerStub) StoreAsync(
	_ context.Context,
	ctxData *MemoryContext,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
	request Request,
) {
	m.storeCalls++
	m.ctxData = ctxData
	m.response = response
	m.messages = append([]llmadapter.Message(nil), messages...)
	m.request = request
}

//nolint:gocritic // Test stub mirrors production interface signature.
func (m *memoryManagerStub) StoreFailureEpisode(
	_ context.Context,
	ctxData *MemoryContext,
	request Request,
	episode FailureEpisode,
) {
	m.failureCalls++
	m.failureCtx = ctxData
	m.failureRequest = request
	m.failureEpisodes = append(m.failureEpisodes, episode)
}

func (m *memoryManagerStub) Compact(
	_ context.Context,
	_ *LoopContext,
	_ telemetry.ContextUsage,
) error {
	m.compactCalls++
	return nil
}

type recordingInvoker struct {
	response *llmadapter.LLMResponse
	err      error
	calls    int
}

func (r *recordingInvoker) Invoke(
	context.Context,
	llmadapter.LLMClient,
	*llmadapter.LLMRequest,
	Request,
) (*llmadapter.LLMResponse, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return r.response, nil
}

func TestConversationLoop_OnEnterAwaitLLM(t *testing.T) {
	t.Run("ShouldInvokeLLMAndEmitResponseEvent", func(t *testing.T) {
		var logBuf bytes.Buffer
		ctx := logger.ContextWithLogger(
			context.Background(),
			logger.NewLogger(&logger.Config{Level: logger.DebugLevel, Output: &logBuf, TimeFormat: "15:04:05"}),
		)
		response := &llmadapter.LLMResponse{}
		inv := &recordingInvoker{response: response}
		loop := newConversationLoop(&settings{maxToolIterations: 3}, noopExecutor{}, noopResponseHandler{}, inv, nil)
		loopCtx := &LoopContext{
			Request:       Request{Agent: &agent.Config{ID: "agent"}, Action: &agent.ActionConfig{ID: "action"}},
			LLMRequest:    &llmadapter.LLMRequest{},
			MaxIterations: 3,
		}
		result := loop.OnEnterAwaitLLM(ctx, loopCtx)
		require.Equal(t, EventLLMResponse, result.Event)
		require.Equal(t, 1, inv.calls)
		require.Equal(t, 1, loopCtx.Iteration)
		require.Same(t, response, loopCtx.Response)
		require.Contains(t, logBuf.String(), "Dispatching LLM request")
	})
	t.Run("ShouldFailWhenIterationBudgetExceeded", func(t *testing.T) {
		ctx := context.Background()
		inv := &recordingInvoker{}
		loop := newConversationLoop(&settings{maxToolIterations: 1}, noopExecutor{}, noopResponseHandler{}, inv, nil)
		loopCtx := &LoopContext{
			LLMRequest:    &llmadapter.LLMRequest{},
			MaxIterations: 1,
			Iteration:     1,
		}
		result := loop.OnEnterAwaitLLM(ctx, loopCtx)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.Contains(t, result.Err.Error(), "max tool iterations")
	})
	t.Run("ShouldFailWhenLoopContextIsNil", func(t *testing.T) {
		loop := newConversationLoop(
			&settings{maxToolIterations: 3},
			noopExecutor{},
			noopResponseHandler{},
			&recordingInvoker{},
			nil,
		)
		result := loop.OnEnterAwaitLLM(context.Background(), nil)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.Contains(t, result.Err.Error(), "loop context is required")
	})
}

func TestConversationLoop_OnEnterEvaluateResponse(t *testing.T) {
	t.Run("ShouldRouteToNoToolWhenNoToolCallsPresent", func(t *testing.T) {
		var logBuf bytes.Buffer
		ctx := logger.ContextWithLogger(
			context.Background(),
			logger.NewLogger(&logger.Config{Level: logger.DebugLevel, Output: &logBuf, TimeFormat: "15:04:05"}),
		)
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			Response:   &llmadapter.LLMResponse{},
		}
		result := loop.OnEnterEvaluateResponse(ctx, loopCtx)
		require.Equal(t, EventResponseNoTool, result.Event)
		require.Contains(t, logBuf.String(), "Evaluating LLM response")
	})
	t.Run("ShouldAppendToolCallsAndRouteToProcessTools", func(t *testing.T) {
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		toolCall := llmadapter.ToolCall{ID: "1", Name: "tool"}
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			Response:   &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{toolCall}},
		}
		result := loop.OnEnterEvaluateResponse(context.Background(), loopCtx)
		require.Equal(t, EventResponseWithTools, result.Event)
		require.Len(t, loopCtx.LLMRequest.Messages, 1)
		require.Equal(t, roleAssistant, loopCtx.LLMRequest.Messages[0].Role)
		require.Equal(t, []llmadapter.ToolCall{toolCall}, loopCtx.LLMRequest.Messages[0].ToolCalls)
	})
	t.Run("ShouldFailWhenResponseMissing", func(t *testing.T) {
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{LLMRequest: &llmadapter.LLMRequest{}}
		result := loop.OnEnterEvaluateResponse(context.Background(), loopCtx)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.Contains(t, result.Err.Error(), "missing LLM response")
	})
	t.Run("ShouldFailWhenLoopContextIsNil", func(t *testing.T) {
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		result := loop.OnEnterEvaluateResponse(context.Background(), nil)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.Contains(t, result.Err.Error(), "loop context is required")
	})
}

func TestConversationLoop_OnEnterProcessTools(t *testing.T) {
	t.Run("ShouldExecuteToolsAndEmitEvent", func(t *testing.T) {
		exec := &processToolsExecutorStub{results: []llmadapter.ToolResult{{ID: "1", Name: "tool", Content: "{}"}}}
		loop := newConversationLoop(&settings{}, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			Response:   &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool"}}},
		}
		result := loop.OnEnterProcessTools(context.Background(), loopCtx)
		require.Equal(t, EventToolsExecuted, result.Event)
		require.NoError(t, result.Err)
		require.Equal(t, 1, exec.calls)
		require.Len(t, exec.lastCalls, 1)
		require.Len(t, loopCtx.ToolResults, 1)
	})
	t.Run("ShouldPropagateExecutionErrors", func(t *testing.T) {
		exec := &processToolsExecutorStub{err: fmt.Errorf("boom")}
		loop := newConversationLoop(&settings{}, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			Response:   &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool"}}},
		}
		result := loop.OnEnterProcessTools(context.Background(), loopCtx)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.EqualError(t, result.Err, "boom")
	})
	t.Run("ShouldFailWhenLoopContextIsNil", func(t *testing.T) {
		exec := &processToolsExecutorStub{}
		loop := newConversationLoop(&settings{}, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		result := loop.OnEnterProcessTools(context.Background(), nil)
		require.Equal(t, EventFailure, result.Event)
		require.Error(t, result.Err)
		require.Contains(t, result.Err.Error(), "loop context is required")
	})
}

func TestConversationLoop_OnEnterUpdateBudgets(t *testing.T) {
	t.Run("ShouldAppendResultsAndEmitBudgetOK", func(t *testing.T) {
		exec := &budgetExecutorStub{}
		cfg := &settings{}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			State:      newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{
				{ID: "1", Name: "tool", Content: "{}"},
			},
			Response: &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool"}}},
		}
		result := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.NoError(t, result.Err)
		require.Equal(t, 1, exec.calls)
		require.Len(t, exec.lastResults, 1)
		require.Len(t, loopCtx.LLMRequest.Messages, 1)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[0].Role)
	})

	t.Run("ShouldAppendEachToolResultAsSeparateMessage", func(t *testing.T) {
		exec := &budgetExecutorStub{}
		cfg := &settings{}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			State:      newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{
				{ID: "1", Name: "first", Content: `{"ok":true}`},
				{ID: "2", Name: "second", Content: `{"ok":false}`},
			},
			Response: &llmadapter.LLMResponse{
				ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "first"}, {ID: "2", Name: "second"}},
			},
		}
		result := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.NoError(t, result.Err)
		require.Equal(t, 1, exec.calls)
		require.Len(t, loopCtx.LLMRequest.Messages, 3)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[0].Role)
		require.Len(t, loopCtx.LLMRequest.Messages[0].ToolResults, 1)
		require.Equal(t, roleTool, loopCtx.LLMRequest.Messages[1].Role)
		require.Len(t, loopCtx.LLMRequest.Messages[1].ToolResults, 1)
		require.Equal(t, roleAssistant, loopCtx.LLMRequest.Messages[2].Role)
		require.Contains(t, loopCtx.LLMRequest.Messages[2].Content, "Observation: tool second failed")
	})
	t.Run("ShouldReturnBudgetExceededWhenUpdateFails", func(t *testing.T) {
		exec := &budgetExecutorStub{err: fmt.Errorf("%w", ErrBudgetExceeded)}
		cfg := &settings{}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest:  &llmadapter.LLMRequest{},
			State:       newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{{ID: "1", Name: "tool"}},
			Response:    &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool"}}},
		}
		result := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetExceeded, result.Event)
		require.Error(t, result.Err)
		require.EqualError(t, result.Err, "budget exceeded")
		require.Equal(t, 1, exec.calls)
		require.Empty(t, loopCtx.LLMRequest.Messages)
	})
	t.Run("ShouldReturnFailureForOperationalErrors", func(t *testing.T) {
		exec := &budgetExecutorStub{err: fmt.Errorf("transient failure")}
		cfg := &settings{}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest:  &llmadapter.LLMRequest{},
			State:       newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{{ID: "1", Name: "tool"}},
			Response:    &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool"}}},
		}
		result := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventFailure, result.Event)
		require.EqualError(t, result.Err, "transient failure")
		require.Equal(t, 1, exec.calls)
		require.Empty(t, loopCtx.LLMRequest.Messages)
	})
	t.Run("ShouldReturnBudgetExceededWhenNoProgressDetected", func(t *testing.T) {
		exec := &budgetExecutorStub{}
		cfg := &settings{enableProgressTracking: true, noProgressThreshold: 1}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			State:      newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{
				{ID: "1", Name: "tool", Content: `{"ok":true}`},
			},
			Response: &llmadapter.LLMResponse{
				ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "tool", Arguments: []byte(`{"arg":1}`)}},
			},
		}
		first := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetOK, first.Event)
		require.NoError(t, first.Err)
		second := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetExceeded, second.Event)
		require.Error(t, second.Err)
		require.ErrorContains(t, second.Err, "no progress")
		require.Equal(t, 2, exec.calls)
	})

	t.Run("ShouldAppendGuidanceMessageWithRemediationHint", func(t *testing.T) {
		exec := &budgetExecutorStub{}
		cfg := &settings{}
		loop := newConversationLoop(cfg, exec, noopResponseHandler{}, &recordingInvoker{}, nil)
		payload := `{"success":false,"error":{"message":"agent_id is required","remediation_hint":"Include \"agent_id\" using a value returned from cp__list_agents before calling cp__call_agent."}}`
		loopCtx := &LoopContext{
			LLMRequest: &llmadapter.LLMRequest{},
			State:      newLoopState(cfg, nil, nil),
			ToolResults: []llmadapter.ToolResult{
				{ID: "1", Name: "cp__call_agent", Content: payload, JSONContent: json.RawMessage(payload)},
			},
			Response: &llmadapter.LLMResponse{
				ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "cp__call_agent"}},
			},
		}
		result := loop.OnEnterUpdateBudgets(context.Background(), loopCtx)
		require.Equal(t, EventBudgetOK, result.Event)
		require.Len(t, loopCtx.LLMRequest.Messages, 2)
		require.Equal(t, roleAssistant, loopCtx.LLMRequest.Messages[1].Role)
		require.Contains(
			t,
			loopCtx.LLMRequest.Messages[1].Content,
			"\"agent_id\" using a value returned from cp__list_agents",
		)
	})
}

func TestConversationLoop_OnEnterHandleCompletion(t *testing.T) {
	t.Run("ShouldReturnRetryWhenHandlerContinues", func(t *testing.T) {
		ctx := context.Background()
		handler := &completionHandlerStub{cont: true}
		loop := newConversationLoop(&settings{}, noopExecutor{}, handler, &recordingInvoker{}, nil)
		state := newLoopState(&settings{}, nil, nil)
		loopCtx := &LoopContext{
			Response:   &llmadapter.LLMResponse{Content: "done"},
			LLMRequest: &llmadapter.LLMRequest{},
			Request:    Request{Action: &agent.ActionConfig{ID: "action"}},
			State:      state,
		}
		result := loop.OnEnterHandleCompletion(ctx, loopCtx)
		require.Equal(t, EventCompletionRetry, result.Event)
		require.NoError(t, result.Err)
		require.Equal(t, 1, handler.calls)
		require.Nil(t, loopCtx.Output)
	})
	t.Run("ShouldReturnSuccessAndSetOutput", func(t *testing.T) {
		ctx := context.Background()
		handler := &completionHandlerStub{output: &core.Output{"answer": "ok"}}
		loop := newConversationLoop(&settings{}, noopExecutor{}, handler, &recordingInvoker{}, nil)
		state := newLoopState(&settings{}, nil, nil)
		loopCtx := &LoopContext{
			Response:   &llmadapter.LLMResponse{Content: "done"},
			LLMRequest: &llmadapter.LLMRequest{},
			Request:    Request{Action: &agent.ActionConfig{ID: "action"}},
			State:      state,
		}
		result := loop.OnEnterHandleCompletion(ctx, loopCtx)
		require.Equal(t, EventCompletionSuccess, result.Event)
		require.NoError(t, result.Err)
		require.Same(t, handler.output, loopCtx.Output)
	})
	t.Run("ShouldPropagateHandlerErrors", func(t *testing.T) {
		ctx := context.Background()
		errBoom := fmt.Errorf("boom")
		handler := &completionHandlerStub{err: errBoom}
		loop := newConversationLoop(&settings{}, noopExecutor{}, handler, &recordingInvoker{}, nil)
		state := newLoopState(&settings{}, nil, nil)
		loopCtx := &LoopContext{
			Response:   &llmadapter.LLMResponse{Content: "done"},
			LLMRequest: &llmadapter.LLMRequest{},
			Request:    Request{Action: &agent.ActionConfig{ID: "action"}},
			State:      state,
		}
		result := loop.OnEnterHandleCompletion(ctx, loopCtx)
		require.Equal(t, EventFailure, result.Event)
		require.EqualError(t, result.Err, "boom")
	})
	t.Run("ShouldFailWhenLoopContextMissingFields", func(t *testing.T) {
		ctx := context.Background()
		loop := newConversationLoop(&settings{}, noopExecutor{}, &completionHandlerStub{}, &recordingInvoker{}, nil)
		res := loop.OnEnterHandleCompletion(ctx, &LoopContext{})
		require.Equal(t, EventFailure, res.Event)
		require.ErrorContains(t, res.Err, "missing LLM response")
	})
	t.Run("ShouldFailWhenLoopContextIsNil", func(t *testing.T) {
		ctx := context.Background()
		loop := newConversationLoop(&settings{}, noopExecutor{}, &completionHandlerStub{}, &recordingInvoker{}, nil)
		res := loop.OnEnterHandleCompletion(ctx, nil)
		require.Equal(t, EventFailure, res.Event)
		require.ErrorContains(t, res.Err, "loop context is required")
	})
}

func TestConversationLoop_OnEnterFinalize(t *testing.T) {
	t.Run("ShouldStoreMemoriesAndEnsureOutput", func(t *testing.T) {
		ctx := context.Background()
		memCtx := &MemoryContext{}
		state := newLoopState(&settings{}, memCtx, nil)
		state.setMemories(memCtx)
		loopCtx := &LoopContext{
			Response: &llmadapter.LLMResponse{Content: "result"},
			State:    state,
			Request:  Request{Agent: &agent.Config{ID: "agent"}, Action: &agent.ActionConfig{ID: "action"}},
			LLMRequest: &llmadapter.LLMRequest{
				Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hello"}},
			},
		}
		memory := &memoryManagerStub{}
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, memory)
		result := loop.OnEnterFinalize(ctx, loopCtx)
		require.Equal(t, "", result.Event)
		require.NoError(t, result.Err)
		require.NotNil(t, loopCtx.Output)
		require.Equal(t, 1, memory.storeCalls)
		require.Equal(t, memCtx, memory.ctxData)
		require.Equal(t, loopCtx.Response, memory.response)
		require.Equal(t, loopCtx.Request, memory.request)
		require.Len(t, memory.messages, 1)
	})
	t.Run("ShouldNotStoreWhenMemoryManagerMissing", func(t *testing.T) {
		ctx := context.Background()
		memCtx := &MemoryContext{}
		state := newLoopState(&settings{}, memCtx, nil)
		loopCtx := &LoopContext{
			Response: &llmadapter.LLMResponse{Content: "result"},
			State:    state,
			Request:  Request{Agent: &agent.Config{ID: "agent"}, Action: &agent.ActionConfig{ID: "action"}},
			LLMRequest: &llmadapter.LLMRequest{
				Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hello"}},
			},
		}
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		result := loop.OnEnterFinalize(ctx, loopCtx)
		require.NoError(t, result.Err)
	})
	t.Run("ShouldFailWhenResponseMissing", func(t *testing.T) {
		ctx := context.Background()
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		res := loop.OnEnterFinalize(ctx, &LoopContext{})
		require.Equal(t, EventFailure, res.Event)
		require.ErrorContains(t, res.Err, "missing LLM response")
	})
	t.Run("ShouldFailWhenLoopContextIsNil", func(t *testing.T) {
		ctx := context.Background()
		loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, nil)
		res := loop.OnEnterFinalize(ctx, nil)
		require.Equal(t, EventFailure, res.Event)
		require.ErrorContains(t, res.Err, "loop context is required")
	})
}

func TestConversationLoop_OnFailureStoresFailureEpisode(t *testing.T) {
	ctx := context.Background()
	memCtx := &MemoryContext{}
	state := newLoopState(&settings{}, memCtx, nil)
	state.setMemories(memCtx)
	loopCtx := &LoopContext{
		Response: &llmadapter.LLMResponse{
			ToolCalls: []llmadapter.ToolCall{
				{
					ID:        "call-1",
					Name:      "cp__call_agent",
					Arguments: json.RawMessage(`{"plan":{"steps":[]}}`),
				},
			},
		},
		State: state,
		Request: Request{
			Agent:  &agent.Config{ID: "agent"},
			Action: &agent.ActionConfig{ID: "action"},
		},
		ToolResults: []llmadapter.ToolResult{
			{
				ID:   "call-1",
				Name: "cp__call_agent",
				JSONContent: json.RawMessage(
					`{"success":false,"error":{"message":"planner produced invalid plan: plan requires at least one step"}}`,
				),
			},
		},
	}
	loopCtx.err = fmt.Errorf("planner produced invalid plan")
	memory := &memoryManagerStub{}
	loop := newConversationLoop(&settings{}, noopExecutor{}, noopResponseHandler{}, &recordingInvoker{}, memory)
	loop.OnFailure(ctx, loopCtx, EventFailure)
	require.Equal(t, 1, memory.failureCalls)
	require.Equal(t, memCtx, memory.failureCtx)
	require.Equal(t, loopCtx.Request, memory.failureRequest)
	require.Len(t, memory.failureEpisodes, 1)
	episode := memory.failureEpisodes[0]
	require.NotEmpty(t, episode.PlanSummary)
	require.NotEmpty(t, episode.ErrorSummary)
	require.Contains(t, episode.ErrorSummary, "plan requires at least one step")
}

type processToolsExecutorStub struct {
	results   []llmadapter.ToolResult
	err       error
	calls     int
	lastCalls []llmadapter.ToolCall
}

func (p *processToolsExecutorStub) Execute(
	_ context.Context,
	calls []llmadapter.ToolCall,
) ([]llmadapter.ToolResult, error) {
	p.calls++
	p.lastCalls = append([]llmadapter.ToolCall(nil), calls...)
	return p.results, p.err
}

func (p *processToolsExecutorStub) UpdateBudgets(context.Context, []llmadapter.ToolResult, *loopState) error {
	return nil
}

type budgetExecutorStub struct {
	err         error
	calls       int
	lastResults []llmadapter.ToolResult
}

func (b *budgetExecutorStub) Execute(context.Context, []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	return nil, nil
}

func (b *budgetExecutorStub) UpdateBudgets(_ context.Context, results []llmadapter.ToolResult, _ *loopState) error {
	b.calls++
	b.lastResults = append([]llmadapter.ToolResult(nil), results...)
	return b.err
}
