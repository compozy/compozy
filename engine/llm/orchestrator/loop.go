package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/llm/usage"
)

const (
	roleUser      = llmadapter.RoleUser
	roleAssistant = "assistant"
	roleTool      = "tool"

	maxFailureGuidanceMessages     = 3
	maxFailureEpisodeText          = 2048
	maxFailureEpisodeCalls         = 4
	maxDynamicExamples             = 3
	compactionFailureWarnThreshold = 3
)

type conversationLoop struct {
	cfg         settings
	tools       ToolExecutor
	responses   ResponseHandler
	invoker     LLMInvoker
	memory      MemoryManager
	middlewares middlewareChain
}

func (l *conversationLoop) refreshUserPrompt(ctx context.Context, loopCtx *LoopContext) error {
	if loopCtx == nil || loopCtx.PromptTemplate == nil || loopCtx.LLMRequest == nil {
		return nil
	}
	if len(loopCtx.LLMRequest.Messages) == 0 {
		return nil
	}
	index := baseUserPromptIndex(loopCtx)
	if index < 0 {
		return nil
	}
	rendered, err := loopCtx.PromptTemplate.Render(ctx, loopCtx.PromptContext)
	if err != nil {
		return err
	}
	combined, _ := combineKnowledgeWithPrompt(rendered, loopCtx.Request.Knowledge.Entries)
	loopCtx.LLMRequest.Messages[index].Content = combined
	return nil
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
	if err := l.refreshUserPrompt(ctx, loopCtx); err != nil {
		return transitionResult{
			Event: EventFailure,
			Err:   fmt.Errorf("failed to render prompt template: %w", err),
		}
	}
	if loopCtx.Iteration >= loopCtx.MaxIterations {
		return transitionResult{
			Event: EventFailure,
			Err:   fmt.Errorf("max tool iterations reached without final response"),
		}
	}
	dynamicPrompt := renderDynamicState(loopCtx, &l.cfg)
	loopCtx.LLMRequest.SystemPrompt = composeSystemPrompt(loopCtx.BaseSystemPrompt, dynamicPrompt)
	if err := l.middlewares.beforeLLMRequest(ctx, loopCtx, loopCtx.LLMRequest); err != nil {
		return transitionResult{Event: EventFailure, Err: err}
	}
	l.logLoopStart(ctx, loopCtx.Request, loopCtx.LLMRequest, loopCtx.Iteration)
	response, err := l.invoker.Invoke(ctx, loopCtx.LLMClient, loopCtx.LLMRequest, loopCtx.Request)
	if err != nil {
		return transitionResult{Event: EventFailure, Err: err}
	}
	if response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm response is required")}
	}
	loopCtx.Response = response
	if err := l.middlewares.afterLLMResponse(ctx, loopCtx, response); err != nil {
		return transitionResult{Event: EventFailure, Err: err}
	}
	loopCtx.Iteration++
	if loopCtx.State != nil {
		loopCtx.State.Iteration.Current = loopCtx.Iteration
	}
	l.recordLLMResponse(ctx, loopCtx, response)
	telemetry.Logger(ctx).Info(
		"LLM response received",
		"iteration", loopCtx.Iteration,
		"tool_calls_count", len(response.ToolCalls),
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
	log := telemetry.Logger(ctx)
	log.Info(
		"Evaluating LLM response",
		"tool_calls_count", len(toolCalls),
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
	log.Info("Routing to process tools", "messages_count", len(loopCtx.LLMRequest.Messages))
	return transitionResult{Event: EventResponseWithTools}
}

func (l *conversationLoop) OnEnterProcessTools(ctx context.Context, loopCtx *LoopContext) transitionResult {
	if loopCtx == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("loop context is required")}
	}
	if loopCtx.Response == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("missing LLM response for tool execution")}
	}
	if loopCtx.LLMRequest == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("llm request is required for tool execution")}
	}
	toolCalls := loopCtx.Response.ToolCalls
	if len(toolCalls) == 0 {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("no tool calls to execute")}
	}
	if l.tools == nil {
		return transitionResult{Event: EventFailure, Err: fmt.Errorf("tool executor is not configured")}
	}
	telemetry.Logger(ctx).Info(
		"Processing tool calls",
		"tool_calls_count", len(toolCalls),
	)
	for i := range toolCalls {
		if err := l.middlewares.beforeToolExecution(ctx, loopCtx, &toolCalls[i]); err != nil {
			return transitionResult{Event: EventFailure, Err: err}
		}
	}
	ctx = llmadapter.ContextWithClient(ctx, loopCtx.LLMClient)
	ctx = llmadapter.ContextWithCallOptions(ctx, &loopCtx.LLMRequest.Options)
	results, err := l.tools.Execute(ctx, toolCalls)
	if err != nil {
		telemetry.Logger(ctx).Error(
			"Tool execution failed",
			"tool_calls_count", len(toolCalls),
			"error", core.RedactError(err),
		)
		return transitionResult{Event: EventFailure, Err: err}
	}
	loopCtx.ToolResults = results
	for i := range loopCtx.ToolResults {
		callPtr := &llmadapter.ToolCall{}
		if i < len(toolCalls) {
			callPtr = &toolCalls[i]
		}
		if err := l.middlewares.afterToolExecution(ctx, loopCtx, callPtr, &loopCtx.ToolResults[i]); err != nil {
			return transitionResult{Event: EventFailure, Err: err}
		}
	}
	telemetry.Logger(ctx).Info(
		"Tool execution complete",
		"results_count", len(results),
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
		if errors.Is(err, ErrBudgetExceeded) {
			telemetry.Logger(ctx).Warn(
				"Tool budget exceeded",
				"error", core.RedactError(err),
			)
			return transitionResult{Event: EventBudgetExceeded, Err: err}
		}
		telemetry.Logger(ctx).Error(
			"Failed to update tool budgets",
			"error", core.RedactError(err),
		)
		return transitionResult{Event: EventFailure, Err: err}
	}
	for i := range loopCtx.ToolResults {
		result := loopCtx.ToolResults[i]
		loopCtx.LLMRequest.Messages = append(
			loopCtx.LLMRequest.Messages,
			llmadapter.Message{Role: roleTool, ToolResults: []llmadapter.ToolResult{result}},
		)
	}
	if l.cfg.enableAgentCallCompletionHints && containsSuccessfulAgentCall(loopCtx.ToolResults) {
		hint := buildAgentCallCompletionHint(loopCtx.ToolResults)
		if hint != "" {
			telemetry.Logger(ctx).Debug("Injecting agent call completion hint", "hint", hint)
			loopCtx.LLMRequest.Messages = append(
				loopCtx.LLMRequest.Messages,
				llmadapter.Message{Role: "user", Content: hint},
			)
		}
	}
	guidanceMessages := buildFailureGuidanceMessages(loopCtx.ToolResults)
	loopCtx.PromptContext.FailureGuidance = extractGuidanceContent(guidanceMessages)
	loopCtx.LLMRequest.Messages = appendFailureGuidanceMessages(
		loopCtx.LLMRequest.Messages,
		guidanceMessages,
	)
	if examples := buildSuccessfulToolExamples(loopCtx.ToolResults); len(examples) > 0 {
		loopCtx.PromptContext.Examples = mergeExamples(loopCtx.PromptContext.Examples, examples)
	}
	if res, handled := l.evaluateProgress(ctx, loopCtx); handled {
		return res
	}
	telemetry.Logger(ctx).Info(
		"Budget evaluation passed",
		"tool_results_count", len(loopCtx.ToolResults),
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
	log := telemetry.Logger(ctx)
	if err != nil {
		log.Error(
			"Completion handling failed",
			"error", core.RedactError(err),
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
		log.Info(
			"Completion succeeded",
			"iteration", loopCtx.Iteration,
			"output_keys", keys,
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
	if err := l.middlewares.beforeFinalize(ctx, loopCtx); err != nil {
		return transitionResult{Event: EventFailure, Err: err}
	}
	if loopCtx.Output == nil {
		output := core.Output{}
		if loopCtx.Response.Content != "" {
			output["response"] = loopCtx.Response.Content
		}
		loopCtx.Output = &output
	}
	if l.memory != nil && loopCtx.State != nil && loopCtx.State.memories() != nil && loopCtx.LLMRequest != nil {
		l.memory.StoreAsync(
			ctx,
			loopCtx.State.memories(),
			loopCtx.Response,
			loopCtx.LLMRequest.Messages,
			loopCtx.Request,
		)
	}
	telemetry.Logger(ctx).Info(
		"Finalizing orchestrator loop",
		"iteration", loopCtx.Iteration,
	)
	return transitionResult{}
}

func (l *conversationLoop) OnEnterTerminateError(context.Context, *LoopContext) transitionResult {
	return transitionResult{}
}

func (l *conversationLoop) OnFailure(ctx context.Context, loopCtx *LoopContext, _ string) {
	if l.memory == nil || loopCtx == nil || loopCtx.State == nil || loopCtx.State.memories() == nil {
		return
	}
	episode := FailureEpisode{
		PlanSummary:  buildFailurePlanSummary(loopCtx.Response),
		ErrorSummary: buildFailureErrorSummary(loopCtx),
	}
	if strings.TrimSpace(episode.PlanSummary) == "" && strings.TrimSpace(episode.ErrorSummary) == "" {
		return
	}
	l.memory.StoreFailureEpisode(ctx, loopCtx.State.memories(), loopCtx.Request, episode)
}

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
	return &conversationLoop{
		cfg:         *cfg,
		tools:       tools,
		responses:   responses,
		invoker:     invoker,
		memory:      memory,
		middlewares: newMiddlewareChain(cfg.middlewares),
	}
}

//nolint:gocritic // Request captured by value to isolate per-run state from upstream mutations.
func (l *conversationLoop) Run(
	ctx context.Context,
	client llmadapter.LLMClient,
	llmReq *llmadapter.LLMRequest,
	request Request,
	state *loopState,
	template PromptTemplateState,
) (*core.Output, *llmadapter.LLMResponse, error) {
	maxIter := l.cfg.maxToolIterations
	if maxIter <= 0 {
		maxIter = defaultMaxToolIterations
	}
	loopCtx := &LoopContext{
		Request:          request,
		LLMClient:        client,
		LLMRequest:       llmReq,
		State:            state,
		MaxIterations:    maxIter,
		BaseSystemPrompt: llmReq.SystemPrompt,
		PromptTemplate:   template,
		PromptContext:    request.Prompt.DynamicContext,
	}
	loopCtx.BaseSystemPrompt = composeSystemPrompt(loopCtx.BaseSystemPrompt, "")
	loopCtx.LLMRequest.SystemPrompt = loopCtx.BaseSystemPrompt
	if loopCtx.State != nil {
		loopCtx.State.Iteration.MaxIterations = loopCtx.MaxIterations
		loopCtx.State.Iteration.Current = loopCtx.Iteration
	}
	loopCtx.baseMessageCount = len(loopCtx.LLMRequest.Messages)
	if l.cfg.restartThresholdClamped {
		telemetry.Logger(ctx).Warn(
			"Restart stall threshold clamped to no-progress threshold",
			"requested_threshold", l.cfg.restartAfterStallRequested,
			"effective_threshold", l.cfg.restartAfterStall,
			"no_progress_threshold", l.cfg.noProgressThreshold,
		)
		telemetry.RecordEvent(ctx, &telemetry.Event{
			Stage:    "restart_threshold_clamped",
			Severity: telemetry.SeverityWarn,
			Metadata: map[string]any{
				"requested_threshold":   l.cfg.restartAfterStallRequested,
				"effective_threshold":   l.cfg.restartAfterStall,
				"no_progress_threshold": l.cfg.noProgressThreshold,
			},
		})
	}
	machine := newLoopFSM(ctx, l, loopCtx)
	if err := machine.Event(ctx, EventStartLoop, loopCtx); err != nil {
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

//nolint:gocritic // Request copied to protect logging fields from concurrent mutation.
func (l *conversationLoop) logLoopStart(ctx context.Context, request Request, llmReq *llmadapter.LLMRequest, iter int) {
	agentID := ""
	if request.Agent != nil {
		agentID = request.Agent.ID
	}
	actionID := ""
	if request.Action != nil {
		actionID = request.Action.ID
	}
	log := telemetry.Logger(ctx)
	log.Info(
		"Dispatching LLM request",
		"agent_id", agentID,
		"action_id", actionID,
		"messages_count", len(llmReq.Messages),
		"tools_count", len(llmReq.Tools),
		"iteration", iter,
	)
	payload := map[string]any{
		"tools": summariseToolDefinitions(llmReq.Tools),
	}
	if telemetry.CaptureContentEnabled(ctx) {
		payload["system_prompt"] = llmReq.SystemPrompt
	} else if llmReq.SystemPrompt != "" {
		payload["system_prompt"] = telemetry.RedactedValue
	}
	payload["messages"] = snapshotMessages(ctx, llmReq.Messages)
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:     "llm_request",
		Severity:  telemetry.SeverityInfo,
		Iteration: iter,
		Metadata: map[string]any{
			"agent_id":       agentID,
			"action_id":      actionID,
			"messages_count": len(llmReq.Messages),
			"tools_count":    len(llmReq.Tools),
		},
		Payload: payload,
	})
}

func (l *conversationLoop) recordLLMResponse(
	ctx context.Context,
	loopCtx *LoopContext,
	response *llmadapter.LLMResponse,
) {
	if response == nil {
		return
	}
	l.recordUsageIfAvailable(ctx, loopCtx, response)
	usage := computeContextUsage(loopCtx, response)
	payload := map[string]any{
		"response": snapshotResponse(ctx, response),
		"usage":    usage,
	}
	if capture := telemetry.CaptureContentEnabled(ctx); !capture && response.Content != "" {
		payload["raw_content"] = telemetry.RedactedValue
	} else if capture {
		payload["raw_content"] = response.Content
	}
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:     "llm_response",
		Severity:  telemetry.SeverityInfo,
		Iteration: loopCtx.Iteration,
		Metadata: map[string]any{
			"tool_calls_count":  len(response.ToolCalls),
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
			"context_limit":     usage.ContextLimit,
			"limit_source":      usage.LimitSource,
			"usage_pct":         usage.PercentOfLimit,
		},
		Payload: payload,
	})
	l.warnIfContextLimitUnknown(ctx, loopCtx, usage)
	if usage.PercentOfLimit > 0 {
		if threshold, ok := telemetry.NotifyContextUsage(ctx, usage.PercentOfLimit); ok {
			telemetry.Logger(ctx).Warn(
				"Context usage threshold crossed",
				"threshold", threshold,
				"usage_pct", usage.PercentOfLimit,
			)
			telemetry.RecordEvent(ctx, &telemetry.Event{
				Stage:     "context_threshold",
				Severity:  telemetry.SeverityWarn,
				Iteration: loopCtx.Iteration,
				Metadata: map[string]any{
					"threshold": threshold,
					"usage_pct": usage.PercentOfLimit,
				},
				Payload: map[string]any{"usage": usage},
			})
			if l.cfg.compactionThreshold > 0 && usage.PercentOfLimit >= l.cfg.compactionThreshold {
				if loopCtx.State != nil {
					loopCtx.State.markCompaction(threshold, usage.PercentOfLimit)
				}
				l.tryCompactMemory(ctx, loopCtx, usage, threshold)
			}
		}
	}
}

func (l *conversationLoop) recordUsageIfAvailable(
	ctx context.Context,
	loopCtx *LoopContext,
	response *llmadapter.LLMResponse,
) {
	collector := usage.FromContext(ctx)
	if collector == nil {
		return
	}
	snapshot, ok := buildUsageSnapshot(loopCtx, response)
	if !ok {
		return
	}
	collector.Record(ctx, &snapshot)
}

func buildUsageSnapshot(loopCtx *LoopContext, response *llmadapter.LLMResponse) (usage.Snapshot, bool) {
	if loopCtx == nil || response == nil || response.Usage == nil {
		return usage.Snapshot{}, false
	}
	provider, model := resolveUsageIdentifiers(loopCtx)
	if provider == "" || model == "" {
		return usage.Snapshot{}, false
	}
	usageMetrics := response.Usage
	return usage.Snapshot{
		Provider:           provider,
		Model:              model,
		PromptTokens:       usageMetrics.PromptTokens,
		CompletionTokens:   usageMetrics.CompletionTokens,
		TotalTokens:        usageMetrics.TotalTokens,
		ReasoningTokens:    usageMetrics.ReasoningTokens,
		CachedPromptTokens: usageMetrics.CachedPromptTokens,
		InputAudioTokens:   usageMetrics.InputAudioTokens,
		OutputAudioTokens:  usageMetrics.OutputAudioTokens,
	}, true
}

// providerMetadataSource exposes provider attribution for dynamically resolved clients.
// Implementations return the logical provider name and model used for the request.
type providerMetadataSource interface {
	ProviderMetadata() (core.ProviderName, string)
}

func resolveUsageIdentifiers(loopCtx *LoopContext) (string, string) {
	if loopCtx == nil {
		return "", ""
	}
	var provider string
	var model string
	if loopCtx.Request.Agent != nil {
		cfg := loopCtx.Request.Agent.Model.Config
		if provider == "" {
			provider = string(cfg.Provider)
		}
		if model == "" {
			model = cfg.Model
		}
	}
	if loopCtx.LLMRequest != nil {
		options := loopCtx.LLMRequest.Options
		if provider == "" {
			provider = string(options.Provider)
		}
		if model == "" {
			model = options.Model
		}
	}
	if source, ok := loopCtx.LLMClient.(providerMetadataSource); ok && source != nil {
		p, m := source.ProviderMetadata()
		if provider == "" {
			provider = string(p)
		}
		if model == "" {
			model = m
		}
	}
	if provider == "" && model == "" {
		return "", ""
	}
	return provider, model
}

func (l *conversationLoop) warnIfContextLimitUnknown(
	ctx context.Context,
	loopCtx *LoopContext,
	usage telemetry.ContextUsage,
) {
	if loopCtx == nil || usage.ContextLimit > 0 || loopCtx.contextLimitWarned {
		return
	}
	provider := ""
	agentID := ""
	actionID := ""
	if loopCtx.Request.Agent != nil {
		agentID = loopCtx.Request.Agent.ID
		provider = string(loopCtx.Request.Agent.Model.Config.Provider)
	}
	if loopCtx.Request.Action != nil {
		actionID = loopCtx.Request.Action.ID
	}
	telemetry.Logger(ctx).Warn(
		"Provider context window unknown; compaction safeguards degraded",
		"agent_id", agentID,
		"action_id", actionID,
		"provider", provider,
	)
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:     "context_limit_unknown",
		Severity:  telemetry.SeverityWarn,
		Iteration: loopCtx.Iteration,
		Metadata: map[string]any{
			"agent_id":     agentID,
			"action_id":    actionID,
			"provider":     provider,
			"limit_source": usage.LimitSource,
		},
	})
	loopCtx.contextLimitWarned = true
}

func (l *conversationLoop) shouldRestart(state *loopState, stallCount int) bool {
	if !l.cfg.enableProgressTracking {
		return false
	}
	if !l.cfg.enableLoopRestarts {
		return false
	}
	if l.cfg.restartAfterStall <= 0 {
		return false
	}
	if stallCount < l.cfg.restartAfterStall {
		return false
	}
	if state == nil {
		return false
	}
	if l.cfg.maxLoopRestarts <= 0 {
		return false
	}
	return state.Iteration.Restarts < l.cfg.maxLoopRestarts
}

func (l *conversationLoop) restartLoop(ctx context.Context, loopCtx *LoopContext, stallCount int) {
	if loopCtx == nil || loopCtx.State == nil {
		return
	}
	restartNumber := loopCtx.State.incrementRestart()
	loopCtx.State.resetBudgets(&l.cfg)
	loopCtx.State.resetProgress()
	loopCtx.State.resetCompaction()
	loopCtx.Response = nil
	loopCtx.ToolResults = nil
	loopCtx.err = nil
	loopCtx.Iteration = 0
	loopCtx.State.Iteration.Current = 0
	loopCtx.PromptContext.FailureGuidance = nil
	loopCtx.PromptContext.Examples = nil
	if loopCtx.LLMRequest != nil {
		loopCtx.LLMRequest.SystemPrompt = loopCtx.BaseSystemPrompt
		if loopCtx.baseMessageCount > 0 && loopCtx.baseMessageCount <= len(loopCtx.LLMRequest.Messages) {
			base, err := llmadapter.CloneMessages(loopCtx.LLMRequest.Messages[:loopCtx.baseMessageCount])
			if err != nil {
				telemetry.Logger(ctx).Error(
					"Failed to clone base messages during loop restart",
					"error", core.RedactError(err),
				)
				base = append([]llmadapter.Message(nil), loopCtx.LLMRequest.Messages[:loopCtx.baseMessageCount]...)
			}
			loopCtx.LLMRequest.Messages = base
		}
	}
	telemetry.Logger(ctx).Warn(
		"Restarting conversation loop due to stalled progress",
		"stall_iterations", stallCount,
		"restart", restartNumber,
		"max_restarts", l.cfg.maxLoopRestarts,
	)
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:    "loop_restart",
		Severity: telemetry.SeverityWarn,
		Metadata: map[string]any{
			"stall_iterations": stallCount,
			"restart_index":    restartNumber,
			"max_restarts":     l.cfg.maxLoopRestarts,
		},
	})
}

func (l *conversationLoop) evaluateProgress(ctx context.Context, loopCtx *LoopContext) (transitionResult, bool) {
	if !l.cfg.enableProgressTracking {
		return transitionResult{}, false
	}
	if loopCtx.Response == nil {
		return transitionResult{
			Event: EventFailure,
			Err:   fmt.Errorf("missing LLM response for progress evaluation"),
		}, true
	}
	fingerprint := buildIterationFingerprint(loopCtx.Response.ToolCalls, loopCtx.ToolResults)
	stallCount := loopCtx.State.recordFingerprint(fingerprint)
	if l.shouldRestart(loopCtx.State, stallCount) {
		l.restartLoop(ctx, loopCtx, stallCount)
		return transitionResult{Event: EventRestartLoop}, true
	}
	if stallCount >= l.cfg.noProgressThreshold && l.cfg.noProgressThreshold > 0 {
		err := fmt.Errorf(
			"%w: %w: %d consecutive iterations",
			ErrBudgetExceeded,
			ErrNoProgress,
			loopCtx.State.Progress.NoProgressCount,
		)
		telemetry.Logger(ctx).Warn(
			"No progress detected",
			"iterations", loopCtx.State.Progress.NoProgressCount,
			"error", core.RedactError(err),
		)
		return transitionResult{Event: EventBudgetExceeded, Err: err}, true
	}
	return transitionResult{}, false
}

func (l *conversationLoop) tryCompactMemory(
	ctx context.Context,
	loopCtx *LoopContext,
	usage telemetry.ContextUsage,
	threshold float64,
) {
	if l.memory == nil || loopCtx == nil || loopCtx.State == nil {
		return
	}
	if l.cfg.compactionThreshold <= 0 {
		return
	}
	if usage.PercentOfLimit < l.cfg.compactionThreshold {
		return
	}
	if !loopCtx.State.compactionPending(loopCtx.Iteration, l.cfg.compactionCooldown) {
		if l.emitCompactionCooldown(ctx, loopCtx, usage, threshold) {
			return
		}
		return
	}
	err := l.memory.Compact(ctx, loopCtx, usage)
	if err == nil {
		loopCtx.State.completeCompaction(loopCtx.Iteration)
		telemetry.Logger(ctx).Info(
			"Memory compaction triggered",
			"usage_pct", usage.PercentOfLimit,
			"threshold", threshold,
		)
		return
	}
	if errors.Is(err, ErrCompactionSkipped) || errors.Is(err, ErrCompactionUnavailable) {
		loopCtx.State.completeCompaction(loopCtx.Iteration)
		telemetry.Logger(ctx).Debug(
			"Memory compaction skipped",
			"usage_pct", usage.PercentOfLimit,
			"threshold", threshold,
		)
		return
	}
	failures := loopCtx.State.recordCompactionFailure(loopCtx.Iteration)
	log := telemetry.Logger(ctx)
	log.Error(
		"Memory compaction failed",
		"error", core.RedactError(err),
		"failures", failures,
	)
	if failures > 0 && failures%compactionFailureWarnThreshold == 0 {
		log.Warn(
			"Repeated memory compaction failures",
			"consecutive_failures", failures,
			"cooldown_iterations", l.cfg.compactionCooldown,
		)
	}
}

func (l *conversationLoop) emitCompactionCooldown(
	ctx context.Context,
	loopCtx *LoopContext,
	usage telemetry.ContextUsage,
	threshold float64,
) bool {
	if loopCtx == nil || loopCtx.State == nil {
		return false
	}
	if !loopCtx.State.Memory.CompactionSuggested {
		return false
	}
	if l.cfg.compactionCooldown <= 0 {
		return false
	}
	remaining := loopCtx.State.Memory.LastCompactionIteration + l.cfg.compactionCooldown - loopCtx.Iteration
	if remaining < 0 {
		remaining = 0
	}
	telemetry.RecordEvent(ctx, &telemetry.Event{
		Stage:     "compaction_cooldown",
		Severity:  telemetry.SeverityInfo,
		Iteration: loopCtx.Iteration,
		Metadata: map[string]any{
			"cooldown_iterations":       l.cfg.compactionCooldown,
			"iterations_until_ready":    remaining,
			"last_compaction_iteration": loopCtx.State.Memory.LastCompactionIteration,
			"usage_pct":                 usage.PercentOfLimit,
			"threshold":                 threshold,
		},
		Payload: map[string]any{"usage": usage},
	})
	return true
}

func summariseToolDefinitions(tools []llmadapter.ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	summary := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		summary = append(summary, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		})
	}
	return summary
}

func snapshotMessages(ctx context.Context, messages []llmadapter.Message) []telemetry.MessageSnapshot {
	if len(messages) == 0 {
		return nil
	}
	capture := telemetry.CaptureContentEnabled(ctx)
	snapshots := make([]telemetry.MessageSnapshot, 0, len(messages))
	for _, msg := range messages {
		snap := telemetry.MessageSnapshot{
			Role:     msg.Role,
			HasParts: len(msg.Parts) > 0,
		}
		if capture {
			snap.Content = msg.Content
		} else if msg.Content != "" {
			snap.Content = telemetry.RedactedValue
		}
		if len(msg.ToolCalls) > 0 {
			snap.ToolCalls = snapshotToolCalls(msg.ToolCalls, capture)
		}
		if len(msg.ToolResults) > 0 {
			snap.ToolResults = snapshotToolResults(msg.ToolResults, capture)
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots
}

func snapshotResponse(ctx context.Context, response *llmadapter.LLMResponse) telemetry.MessageSnapshot {
	if response == nil {
		return telemetry.MessageSnapshot{}
	}
	msg := llmadapter.Message{
		Role:      roleAssistant,
		Content:   response.Content,
		ToolCalls: response.ToolCalls,
	}
	snaps := snapshotMessages(ctx, []llmadapter.Message{msg})
	if len(snaps) == 0 {
		return telemetry.MessageSnapshot{}
	}
	return snaps[0]
}

func snapshotToolCalls(calls []llmadapter.ToolCall, capture bool) []telemetry.ToolCallSnapshot {
	if len(calls) == 0 {
		return nil
	}
	out := make([]telemetry.ToolCallSnapshot, 0, len(calls))
	for _, c := range calls {
		snap := telemetry.ToolCallSnapshot{
			ID:   c.ID,
			Name: c.Name,
		}
		if capture {
			snap.Arguments = c.Arguments
		}
		out = append(out, snap)
	}
	return out
}

func snapshotToolResults(results []llmadapter.ToolResult, capture bool) []telemetry.ToolResultSnapshot {
	if len(results) == 0 {
		return nil
	}
	out := make([]telemetry.ToolResultSnapshot, 0, len(results))
	for _, r := range results {
		snap := telemetry.ToolResultSnapshot{
			ID:   r.ID,
			Name: r.Name,
		}
		if capture {
			snap.Content = r.Content
			snap.JSONContent = r.JSONContent
		} else if r.Content != "" {
			snap.Content = telemetry.RedactedValue
		}
		out = append(out, snap)
	}
	return out
}

func computeContextUsage(loopCtx *LoopContext, response *llmadapter.LLMResponse) telemetry.ContextUsage {
	if loopCtx == nil || response == nil || response.Usage == nil {
		return telemetry.ContextUsage{}
	}
	usage := response.Usage
	total := usage.TotalTokens
	if total == 0 {
		total = usage.PromptTokens + usage.CompletionTokens
	}
	limit := loopCtx.Request.Execution.ProviderCaps.ContextWindowTokens
	source := "provider"
	if limit <= 0 && loopCtx.Request.Agent != nil && loopCtx.Request.Agent.Model.Config.Params.MaxTokens > 0 {
		limit = int(loopCtx.Request.Agent.Model.Config.Params.MaxTokens)
		source = "agent_max_tokens"
	}
	if limit <= 0 && loopCtx.LLMRequest != nil && loopCtx.LLMRequest.Options.MaxTokens > 0 {
		limit = int(loopCtx.LLMRequest.Options.MaxTokens)
		source = "request_max_tokens"
	}
	if limit <= 0 {
		source = "unknown"
	}
	percent := 0.0
	if limit > 0 && total > 0 {
		percent = float64(total) / float64(limit)
	}
	return telemetry.ContextUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      total,
		ContextLimit:     limit,
		LimitSource:      source,
		PercentOfLimit:   percent,
	}
}

func baseUserPromptIndex(loopCtx *LoopContext) int {
	if loopCtx == nil || loopCtx.LLMRequest == nil {
		return -1
	}
	messages := loopCtx.LLMRequest.Messages
	if len(messages) == 0 {
		return -1
	}
	limit := loopCtx.baseMessageCount
	if limit <= 0 || limit > len(messages) {
		limit = len(messages)
	}
	for i := limit - 1; i >= 0; i-- {
		if messages[i].Role == roleUser {
			return i
		}
	}
	return -1
}

func buildFailurePlanSummary(resp *llmadapter.LLMResponse) string {
	if resp == nil {
		return ""
	}
	if content := truncateEpisodeText(resp.Content); content != "" {
		return content
	}
	if len(resp.ToolCalls) == 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString("Tool plan:")
	for i, call := range resp.ToolCalls {
		if i >= maxFailureEpisodeCalls {
			break
		}
		builder.WriteString("\n- ")
		builder.WriteString(call.Name)
		if args := strings.TrimSpace(string(call.Arguments)); args != "" {
			builder.WriteString(" ")
			builder.WriteString(core.RedactString(args))
		}
	}
	if remaining := len(resp.ToolCalls) - maxFailureEpisodeCalls; remaining > 0 {
		builder.WriteString(fmt.Sprintf("\n... %d more tool call(s)", remaining))
	}
	return truncateEpisodeText(builder.String())
}

func buildFailureErrorSummary(loopCtx *LoopContext) string {
	if loopCtx == nil {
		return ""
	}
	if guidance := buildFailureGuidanceMessages(loopCtx.ToolResults); len(guidance) > 0 {
		return truncateEpisodeText(guidance[len(guidance)-1].Content)
	}
	if loopCtx.err != nil {
		return truncateEpisodeText("Error: " + core.RedactError(loopCtx.err))
	}
	return ""
}

func truncateEpisodeText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxFailureEpisodeText {
		return trimmed
	}
	return string(runes[:maxFailureEpisodeText])
}

func extractGuidanceContent(messages []llmadapter.Message) []string {
	if len(messages) == 0 {
		return nil
	}
	out := make([]string, 0, len(messages))
	for i := range messages {
		text := strings.TrimSpace(messages[i].Content)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func buildSuccessfulToolExamples(results []llmadapter.ToolResult) []PromptExample {
	examples := make([]PromptExample, 0, len(results))
	for _, result := range results {
		if !isToolResultSuccess(result) {
			continue
		}
		payload := toolResultExamplePayload(result)
		if payload == "" {
			continue
		}
		summary := fmt.Sprintf("Successful tool call %s", result.Name)
		content := core.RedactString(truncateEpisodeText(payload))
		if content == "" {
			continue
		}
		examples = append(examples, PromptExample{Summary: summary, Content: content})
		if len(examples) >= maxDynamicExamples {
			break
		}
	}
	return examples
}

func mergeExamples(base, extra []PromptExample) []PromptExample {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := make([]PromptExample, 0, len(base)+len(extra))
	for _, ex := range base {
		ex := PromptExample{Summary: strings.TrimSpace(ex.Summary), Content: strings.TrimSpace(ex.Content)}
		if ex.Summary == "" && ex.Content == "" {
			continue
		}
		if exampleExists(merged, ex) {
			continue
		}
		merged = append(merged, ex)
	}
	for _, ex := range extra {
		ex := PromptExample{Summary: strings.TrimSpace(ex.Summary), Content: strings.TrimSpace(ex.Content)}
		if ex.Summary == "" && ex.Content == "" {
			continue
		}
		if exampleExists(merged, ex) {
			continue
		}
		merged = append(merged, ex)
	}
	if len(merged) > maxDynamicExamples {
		merged = merged[len(merged)-maxDynamicExamples:]
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func exampleExists(list []PromptExample, candidate PromptExample) bool {
	for _, item := range list {
		if item.Summary == candidate.Summary && item.Content == candidate.Content {
			return true
		}
	}
	return false
}

func toolResultExamplePayload(result llmadapter.ToolResult) string {
	if len(result.JSONContent) > 0 {
		return strings.TrimSpace(string(result.JSONContent))
	}
	return strings.TrimSpace(result.Content)
}

func appendFailureGuidanceMessages(
	messages []llmadapter.Message,
	guidance []llmadapter.Message,
) []llmadapter.Message {
	if len(guidance) == 0 {
		return messages
	}
	filtered := make([]llmadapter.Message, 0, len(messages))
	existingGuidance := make([]llmadapter.Message, 0)
	for i := range messages {
		if isFailureGuidanceMessage(&messages[i]) {
			existingGuidance = append(existingGuidance, messages[i])
			continue
		}
		filtered = append(filtered, messages[i])
	}
	existingGuidance = append(existingGuidance, guidance...)
	if len(existingGuidance) > maxFailureGuidanceMessages {
		existingGuidance = existingGuidance[len(existingGuidance)-maxFailureGuidanceMessages:]
	}
	return append(filtered, existingGuidance...)
}

func isFailureGuidanceMessage(msg *llmadapter.Message) bool {
	if msg == nil {
		return false
	}
	return msg.Role == roleAssistant &&
		strings.HasPrefix(msg.Content, "Observation: tool ") &&
		len(msg.ToolCalls) == 0 &&
		len(msg.ToolResults) == 0
}
