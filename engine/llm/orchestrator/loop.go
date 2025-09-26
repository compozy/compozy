package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	roleAssistant = "assistant"
	roleTool      = "tool"
)

type conversationLoop struct {
	cfg       settings
	tools     ToolExecutor
	responses ResponseHandler
	invoker   LLMInvoker
	memory    MemoryManager
}

func (l *conversationLoop) OnEnterInit(context.Context, *LoopContext) transitionResult {
	return transitionResult{}
}

func (l *conversationLoop) OnEnterAwaitLLM(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if loopCtx.LLMRequest == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm request is required")}
	}
	if loopCtx.Iteration >= loopCtx.MaxIterations {
		return transitionResult{
			Event: EventFailure,
			Err:   fmt.Errorf("max tool iterations reached without final response"),
		}
	}
	l.logLoopStart(ctx, loopCtx.Request, loopCtx.LLMRequest, loopCtx.Iteration)
	response, err := l.invoker.Invoke(ctx, loopCtx.LLMClient, loopCtx.LLMRequest, loopCtx.Request)
	if err != nil {
		return transitionResult{Event: EventFailure, Err: err}
	}
	loopCtx.Response = response
	loopCtx.Iteration++
	logger.FromContext(ctx).Debug(
		"LLM response received",
		"iteration",
		loopCtx.Iteration,
		"tool_calls_count",
		len(response.ToolCalls),
	)
	return transitionResult{Event: EventLLMResponse, Args: []any{response}}
}

func (l *conversationLoop) OnEnterEvaluateResponse(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	response := loopCtx.Response
	if response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("missing LLM response")}
	}
	toolCalls := response.ToolCalls
	log := logger.FromContext(ctx)
	log.Debug(
		"Evaluating LLM response",
		"tool_calls_count",
		len(toolCalls),
	)
	if len(toolCalls) == 0 {
		return transitionResult{Event: EventResponseNoTool}
	}
	if loopCtx.LLMRequest == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm request is required to append tool calls")}
	}
	loopCtx.LLMRequest.Messages = append(
		loopCtx.LLMRequest.Messages,
		llmadapter.Message{Role: roleAssistant, ToolCalls: toolCalls},
	)
	log.Debug("Routing to process tools", "messages_count", len(loopCtx.LLMRequest.Messages))
	return transitionResult{Event: EventResponseWithTools}
}

func (l *conversationLoop) OnEnterProcessTools(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if loopCtx.Response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("missing LLM response for tool execution")}
	}
	toolCalls := loopCtx.Response.ToolCalls
	if len(toolCalls) == 0 {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("no tool calls to execute")}
	}
	if l.tools == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("tool executor is not configured")}
	}
	logger.FromContext(ctx).Debug(
		"Processing tool calls",
		"tool_calls_count",
		len(toolCalls),
	)
	results, err := l.tools.Execute(ctx, toolCalls)
	if err != nil {
		logger.FromContext(ctx).Error(
			"Tool execution failed",
			"tool_calls_count",
			len(toolCalls),
			"error",
			core.RedactError(err),
		)
		return transitionResult{Event: EventFailure, Err: err}
	}
	loopCtx.ToolResults = results
	logger.FromContext(ctx).Debug(
		"Tool execution complete",
		"results_count",
		len(results),
	)
	return transitionResult{Event: EventToolsExecuted, Args: []any{results}}
}

func (l *conversationLoop) OnEnterUpdateBudgets(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if l.tools == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("tool executor is not configured")}
	}
	if loopCtx.State == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop state is required for budget evaluation")}
	}
	if loopCtx.LLMRequest == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm request is required to append tool results")}
	}
	if err := l.tools.UpdateBudgets(ctx, loopCtx.ToolResults, loopCtx.State); err != nil {
		logger.FromContext(ctx).Warn(
			"Tool budget exceeded",
			"error",
			core.RedactError(err),
		)
		return transitionResult{Event: EventBudgetExceeded, Err: err}
	}
	loopCtx.LLMRequest.Messages = append(
		loopCtx.LLMRequest.Messages,
		llmadapter.Message{Role: roleTool, ToolResults: loopCtx.ToolResults},
	)
	if l.cfg.enableProgressTracking {
		if loopCtx.Response == nil {
			return transitionResult{
				Event: EventFailure,
				Err:   fmt.Errorf("missing LLM response for progress evaluation"),
			}
		}
		fingerprint := buildIterationFingerprint(loopCtx.Response.ToolCalls, loopCtx.ToolResults)
		if loopCtx.State.detectNoProgress(l.cfg.noProgressThreshold, fingerprint) {
			err := fmt.Errorf("%w: %d consecutive iterations", ErrNoProgress, loopCtx.State.noProgressCount)
			logger.FromContext(ctx).Warn(
				"No progress detected",
				"iterations",
				loopCtx.State.noProgressCount,
				"error",
				core.RedactError(err),
			)
			return transitionResult{Event: EventBudgetExceeded, Err: err}
		}
	}
	logger.FromContext(ctx).Debug(
		"Budget evaluation passed",
		"tool_results_count",
		len(loopCtx.ToolResults),
	)
	return transitionResult{Event: EventBudgetOK}
}

func (l *conversationLoop) OnEnterHandleCompletion(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if loopCtx.Response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("missing LLM response for completion handling")}
	}
	if loopCtx.LLMRequest == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm request is required for completion handling")}
	}
	if loopCtx.State == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop state is required for completion handling")}
	}
	event, err := l.handleCompletion(ctx, loopCtx)
	log := logger.FromContext(ctx)
	if err != nil {
		log.Error(
			"Completion handling failed",
			"error",
			core.RedactError(err),
		)
		return transitionResult{Event: event, Err: err}
	}
	switch event {
	case EventCompletionRetry:
		log.Debug(
			"Completion requires retry",
			"iteration",
			loopCtx.Iteration,
		)
	case EventCompletionSuccess:
		keys := 0
		if loopCtx.Output != nil {
			keys = len(*loopCtx.Output)
		}
		log.Debug(
			"Completion succeeded",
			"iteration",
			loopCtx.Iteration,
			"output_keys",
			keys,
		)
	}
	return transitionResult{Event: event}
}

func (l *conversationLoop) OnEnterFinalize(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if loopCtx.Response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("missing LLM response for finalize")}
	}
	if loopCtx.Output == nil {
		output := core.Output{}
		if loopCtx.Response.Content != "" {
			output["response"] = loopCtx.Response.Content
		}
		loopCtx.Output = &output
	}
	if l.memory != nil && loopCtx.State != nil && loopCtx.State.memories != nil && loopCtx.LLMRequest != nil {
		l.memory.StoreAsync(ctx, loopCtx.State.memories, loopCtx.Response, loopCtx.LLMRequest.Messages, loopCtx.Request)
	}
	logger.FromContext(ctx).Debug(
		"Finalizing orchestrator loop",
		"iteration",
		loopCtx.Iteration,
	)
	return transitionResult{}
}

func (l *conversationLoop) OnEnterTerminateError(context.Context, *LoopContext) transitionResult {
	return transitionResult{}
}

func (l *conversationLoop) OnFailure(context.Context, *LoopContext, string) {}

func newConversationLoop(
	cfg *settings,
	tools ToolExecutor,
	responses ResponseHandler,
	invoker LLMInvoker,
	memory MemoryManager,
) *conversationLoop {
	if cfg == nil {
		cfg = &settings{}
	}
	return &conversationLoop{cfg: *cfg, tools: tools, responses: responses, invoker: invoker, memory: memory}
}

func (l *conversationLoop) Run(
	ctx context.Context,
	client llmadapter.LLMClient,
	llmReq *llmadapter.LLMRequest,
	request Request,
	state *loopState,
) (*core.Output, *llmadapter.LLMResponse, error) {
	maxIter := l.cfg.maxToolIterations
	if maxIter <= 0 {
		maxIter = defaultMaxToolIterations
	}
	loopCtx := &LoopContext{
		Request:       request,
		LLMClient:     client,
		LLMRequest:    llmReq,
		State:         state,
		MaxIterations: maxIter,
	}
	machine := newLoopFSM(ctx, l, loopCtx)
	if err := machine.Event(ctx, EventStartLoop); err != nil {
		if loopCtx.err != nil {
			return nil, nil, loopCtx.err
		}
		return nil, nil, err
	}
	switch machine.Current() {
	case StateFinalize:
		return loopCtx.Output, loopCtx.Response, nil
	case StateTerminateError:
		if loopCtx.err != nil {
			return nil, nil, loopCtx.err
		}
		return nil, nil, fmt.Errorf("state machine terminated without context")
	default:
		return nil, nil, fmt.Errorf("unsupported state: %s", machine.Current())
	}
}

func (l *conversationLoop) handleCompletion(ctx context.Context, loopCtx *LoopContext) (string, error) {
	output, cont, err := l.responses.HandleNoToolCalls(
		ctx,
		loopCtx.Response,
		loopCtx.Request,
		loopCtx.LLMRequest,
		loopCtx.State,
	)
	if err != nil {
		return EventFailure, err
	}
	if cont {
		return EventCompletionRetry, nil
	}
	loopCtx.Output = output
	return EventCompletionSuccess, nil
}

func (l *conversationLoop) logLoopStart(ctx context.Context, request Request, llmReq *llmadapter.LLMRequest, iter int) {
	agentID := ""
	if request.Agent != nil {
		agentID = request.Agent.ID
	}
	actionID := ""
	if request.Action != nil {
		actionID = request.Action.ID
	}
	logger.FromContext(ctx).Debug(
		"Generating LLM response",
		"agent_id", agentID,
		"action_id", actionID,
		"messages_count", len(llmReq.Messages),
		"tools_count", len(llmReq.Tools),
		"iteration", iter,
	)
}
