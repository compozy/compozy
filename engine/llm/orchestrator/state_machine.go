package orchestrator

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/looplab/fsm"
)

const (
	StateInit             = "init"
	StateAwaitLLM         = "await_llm"
	StateEvaluateResponse = "evaluate_response"
	StateProcessTools     = "process_tools"
	StateUpdateBudgets    = "update_budgets"
	StateHandleCompletion = "handle_completion"
	StateFinalize         = "finalize"
	StateTerminateError   = "terminate_error"
)

const (
	EventStartLoop         = "start_loop"
	EventLLMResponse       = "llm_response"
	EventResponseNoTool    = "response_no_tool"
	EventResponseWithTools = "response_with_tools"
	EventToolsExecuted     = "tools_executed"
	EventBudgetOK          = "budget_ok"
	EventBudgetExceeded    = "budget_exceeded"
	EventCompletionRetry   = "completion_retry"
	EventCompletionSuccess = "completion_success"
	EventFailure           = "failure"
)

type loopDeps interface {
	OnEnterInit(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterAwaitLLM(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterEvaluateResponse(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterProcessTools(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterUpdateBudgets(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterHandleCompletion(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterFinalize(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnEnterTerminateError(ctx context.Context, loopCtx *LoopContext) transitionResult
	OnFailure(ctx context.Context, loopCtx *LoopContext, event string)
}

type transitionResult struct {
	Event string
	Args  []any
	Err   error
}

type LoopContext struct {
	Request        Request
	LLMClient      llmadapter.LLMClient
	LLMRequest     *llmadapter.LLMRequest
	State          *loopState
	Response       *llmadapter.LLMResponse
	ToolResults    []llmadapter.ToolResult
	Output         *core.Output
	Iteration      int
	MaxIterations  int
	err            error
	eventStartedAt time.Time
}

func newLoopFSM(ctx context.Context, deps loopDeps, _ *LoopContext) *fsm.FSM {
	observer := newTransitionObserver(ctx)
	machine := fsm.NewFSM(
		StateInit,
		loopFSMEvents(),
		loopFSMCallbacks(observer, deps),
	)
	return machine
}

func loopFSMEvents() fsm.Events {
	return fsm.Events{
		{Name: EventStartLoop, Src: []string{StateInit}, Dst: StateAwaitLLM},
		{Name: EventLLMResponse, Src: []string{StateAwaitLLM}, Dst: StateEvaluateResponse},
		{Name: EventResponseNoTool, Src: []string{StateEvaluateResponse}, Dst: StateHandleCompletion},
		{Name: EventResponseWithTools, Src: []string{StateEvaluateResponse}, Dst: StateProcessTools},
		{Name: EventToolsExecuted, Src: []string{StateProcessTools}, Dst: StateUpdateBudgets},
		{Name: EventBudgetOK, Src: []string{StateUpdateBudgets}, Dst: StateAwaitLLM},
		{Name: EventBudgetExceeded, Src: []string{StateUpdateBudgets}, Dst: StateTerminateError},
		{Name: EventCompletionRetry, Src: []string{StateHandleCompletion}, Dst: StateAwaitLLM},
		{Name: EventCompletionSuccess, Src: []string{StateHandleCompletion}, Dst: StateFinalize},
		{
			Name: EventFailure,
			Src: []string{
				StateAwaitLLM,
				StateEvaluateResponse,
				StateProcessTools,
				StateHandleCompletion,
				StateUpdateBudgets,
			},
			Dst: StateTerminateError,
		},
	}
}

func loopFSMCallbacks(observer *transitionObserver, deps loopDeps) fsm.Callbacks {
	callbacks := fsm.Callbacks{
		"before_event": func(cbCtx context.Context, e *fsm.Event) { observer.BeforeEvent(cbCtx, e) },
		"after_event":  func(cbCtx context.Context, e *fsm.Event) { observer.AfterEvent(cbCtx, e) },
		"after_" + EventFailure: func(cbCtx context.Context, e *fsm.Event) {
			if deps == nil {
				return
			}
			obsCtx := observer.resolveContext(cbCtx)
			deps.OnFailure(obsCtx, loopContextFromEvent(obsCtx, e), e.Event)
		},
	}
	callbacks["enter_"+StateInit] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterInit(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateAwaitLLM] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterAwaitLLM(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateEvaluateResponse] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterEvaluateResponse(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateProcessTools] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterProcessTools(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateUpdateBudgets] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterUpdateBudgets(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateHandleCompletion] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterHandleCompletion(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateFinalize] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterFinalize(cbCtx, lc)
		},
	)
	callbacks["enter_"+StateTerminateError] = makeEnterCallback(
		observer,
		deps,
		func(d loopDeps, cbCtx context.Context, lc *LoopContext) transitionResult {
			return d.OnEnterTerminateError(cbCtx, lc)
		},
	)
	return callbacks
}

func loopContextFromEvent(ctx context.Context, e *fsm.Event) *LoopContext {
	resolvedCtx := ctx
	if resolvedCtx == nil {
		resolvedCtx = context.TODO()
	}
	if e == nil {
		logger.FromContext(resolvedCtx).Error("FSM loop context lookup failed", "reason", "nil event")
		return &LoopContext{}
	}
	if len(e.Args) > 0 {
		if lc, ok := e.Args[0].(*LoopContext); ok && lc != nil {
			return lc
		}
	}
	logger.FromContext(resolvedCtx).Error("FSM loop context missing from event args", "event", e.Event)
	return &LoopContext{}
}

func applyTransitionResult(ctx context.Context, observer *transitionObserver, e *fsm.Event, result transitionResult) {
	if result.Event == "" && result.Err == nil {
		return
	}
	resolvedCtx := observer.resolveContext(ctx)
	loopCtx := loopContextFromEvent(resolvedCtx, e)
	if result.Err != nil {
		loopCtx.err = result.Err
		if result.Event == "" {
			result.Event = EventFailure
		}
	}
	if result.Event == "" {
		return
	}
	transitionCtx := observer.resolveContext(ctx)
	args := append([]any{loopCtx}, result.Args...)
	if err := e.FSM.Event(transitionCtx, result.Event, args...); err != nil && loopCtx.err == nil {
		loopCtx.err = err
	}
}

type transitionObserver struct {
	now     func() time.Time
	baseCtx context.Context
}

func newTransitionObserver(ctx context.Context) *transitionObserver {
	return &transitionObserver{now: time.Now, baseCtx: ctx}
}

func (o *transitionObserver) resolveContext(cbCtx context.Context) context.Context {
	if cbCtx != nil {
		return cbCtx
	}
	if o != nil && o.baseCtx != nil {
		return o.baseCtx
	}
	return context.TODO()
}

func (o *transitionObserver) BeforeEvent(cbCtx context.Context, e *fsm.Event) {
	resolvedCtx := o.resolveContext(cbCtx)
	loopCtx := loopContextFromEvent(resolvedCtx, e)
	loopCtx.eventStartedAt = o.now()
	logger.FromContext(resolvedCtx).Debug(
		"FSM transition start",
		"event", e.Event,
		"from_state", e.Src,
		"to_state", e.Dst,
		"iteration", loopCtx.Iteration,
	)
}

func (o *transitionObserver) AfterEvent(cbCtx context.Context, e *fsm.Event) {
	resolvedCtx := o.resolveContext(cbCtx)
	loopCtx := loopContextFromEvent(resolvedCtx, e)
	duration := time.Duration(0)
	if !loopCtx.eventStartedAt.IsZero() {
		duration = o.now().Sub(loopCtx.eventStartedAt)
	}
	keyvals := []any{
		"event", e.Event,
		"from_state", e.Src,
		"to_state", e.Dst,
		"iteration", loopCtx.Iteration,
	}
	if duration > 0 {
		keyvals = append(keyvals, "duration_ms", duration.Milliseconds())
	}
	if loopCtx.err != nil {
		keyvals = append(keyvals, "error", core.RedactError(loopCtx.err))
	}
	logger.FromContext(resolvedCtx).Debug(
		"FSM transition complete",
		keyvals...,
	)
}

func (o *transitionObserver) EnterState(cbCtx context.Context, e *fsm.Event, loopCtx *LoopContext) {
	resolvedCtx := o.resolveContext(cbCtx)
	logger.FromContext(resolvedCtx).Debug(
		"FSM state entered",
		"state", e.Dst,
		"event", e.Event,
		"iteration", loopCtx.Iteration,
	)
}

func makeEnterCallback(
	observer *transitionObserver,
	deps loopDeps,
	handler func(loopDeps, context.Context, *LoopContext) transitionResult,
) fsm.Callback {
	return func(cbCtx context.Context, e *fsm.Event) {
		callCtx := observer.resolveContext(cbCtx)
		loopCtx := loopContextFromEvent(callCtx, e)
		observer.EnterState(callCtx, e, loopCtx)
		if deps == nil {
			return
		}
		applyTransitionResult(callCtx, observer, e, handler(deps, callCtx, loopCtx))
	}
}
