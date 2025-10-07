package orchestrate

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/looplab/fsm"
)

const (
	StatePending         = "pending"
	StatePlanning        = "planning"
	StateDispatching     = "dispatching"
	StateAwaitingResults = "awaiting_results"
	StateMerging         = "merging"
	StateCompleted       = "completed"
	StateFailed          = "failed"
)

const (
	EventStartPlan        = "start_plan"
	EventPlannerFinished  = "planner_finished"
	EventValidationFailed = "validation_failed"
	EventDispatchStep     = "dispatch_step"
	EventStepSucceeded    = "step_succeeded"
	EventStepFailed       = "step_failed"
	EventParallelComplete = "parallel_complete"
	EventTimeout          = "timeout"
	EventPanic            = "panic"
)

type executorDeps interface {
	OnEnterPending(context.Context, *executorContext) TransitionResult
	OnEnterPlanning(context.Context, *executorContext) TransitionResult
	OnEnterDispatching(context.Context, *executorContext) TransitionResult
	OnEnterAwaitingResults(context.Context, *executorContext) TransitionResult
	OnEnterMerging(context.Context, *executorContext) TransitionResult
	OnEnterCompleted(context.Context, *executorContext) TransitionResult
	OnEnterFailed(context.Context, *executorContext) TransitionResult
	OnFailure(context.Context, *executorContext, string)
}

type TransitionResult struct {
	Event string
	Args  []any
	Err   error
}

func newExecutorFSM(ctx context.Context, deps executorDeps, _ *executorContext) *fsm.FSM {
	observer := newExecutorObserver(ctx)
	return fsm.NewFSM(StatePending, executorFSMEvents(), executorFSMCallbacks(observer, deps))
}

func executorFSMEvents() fsm.Events {
	return fsm.Events{
		{Name: EventStartPlan, Src: []string{StatePending}, Dst: StatePlanning},
		{Name: EventPlannerFinished, Src: []string{StatePlanning, StateMerging}, Dst: StateDispatching},
		{Name: EventValidationFailed, Src: []string{StatePlanning}, Dst: StateFailed},
		{Name: EventDispatchStep, Src: []string{StateDispatching}, Dst: StateAwaitingResults},
		{Name: EventStepSucceeded, Src: []string{StateAwaitingResults}, Dst: StateMerging},
		{Name: EventParallelComplete, Src: []string{StateAwaitingResults}, Dst: StateMerging},
		{Name: EventParallelComplete, Src: []string{StateMerging}, Dst: StateCompleted},
		{Name: EventParallelComplete, Src: []string{StateDispatching}, Dst: StateCompleted},
		{Name: EventStepFailed, Src: []string{StateAwaitingResults, StateDispatching}, Dst: StateFailed},
		{Name: EventTimeout, Src: []string{StateAwaitingResults, StateDispatching}, Dst: StateFailed},
		{Name: EventPanic, Src: []string{StateAwaitingResults, StateDispatching}, Dst: StateFailed},
	}
}

func executorFSMCallbacks(observer *executorObserver, deps executorDeps) fsm.Callbacks {
	callbacks := fsm.Callbacks{
		"before_event": func(cbCtx context.Context, e *fsm.Event) {
			observer.BeforeEvent(cbCtx, e)
		},
		"after_event": func(cbCtx context.Context, e *fsm.Event) {
			observer.AfterEvent(cbCtx, e)
		},
	}
	callbacks["after_"+EventStepFailed] = makeFailureCallback(observer, deps)
	callbacks["after_"+EventValidationFailed] = makeFailureCallback(observer, deps)
	callbacks["after_"+EventTimeout] = makeFailureCallback(observer, deps)
	callbacks["after_"+EventPanic] = makeFailureCallback(observer, deps)
	register := func(state string, handler func(executorDeps, context.Context, *executorContext) TransitionResult) {
		callbacks["enter_"+state] = makeEnterCallback(observer, deps, handler)
	}
	register(StatePending, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterPending(cbCtx, ec)
	})
	register(StatePlanning, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterPlanning(cbCtx, ec)
	})
	register(StateDispatching, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterDispatching(cbCtx, ec)
	})
	register(StateAwaitingResults, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterAwaitingResults(cbCtx, ec)
	})
	register(StateMerging, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterMerging(cbCtx, ec)
	})
	register(StateCompleted, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterCompleted(cbCtx, ec)
	})
	register(StateFailed, func(d executorDeps, cbCtx context.Context, ec *executorContext) TransitionResult {
		return d.OnEnterFailed(cbCtx, ec)
	})
	return callbacks
}

func makeFailureCallback(observer *executorObserver, deps executorDeps) fsm.Callback {
	return func(cbCtx context.Context, e *fsm.Event) {
		if deps == nil {
			return
		}
		reportCtx := observer.resolveContext(cbCtx)
		execCtx := executorContextFromEvent(reportCtx, e)
		deps.OnFailure(reportCtx, execCtx, e.Event)
	}
}

func makeEnterCallback(
	observer *executorObserver,
	deps executorDeps,
	handler func(executorDeps, context.Context, *executorContext) TransitionResult,
) fsm.Callback {
	return func(cbCtx context.Context, e *fsm.Event) {
		callCtx := observer.resolveContext(cbCtx)
		execCtx := executorContextFromEvent(callCtx, e)
		observer.EnterState(callCtx, e, execCtx)
		if deps == nil {
			return
		}
		applyTransitionResult(callCtx, observer, e, handler(deps, callCtx, execCtx))
	}
}

func applyTransitionResult(ctx context.Context, observer *executorObserver, e *fsm.Event, result TransitionResult) {
	if result.Event == "" && result.Err == nil {
		return
	}
	resolvedCtx := observer.resolveContext(ctx)
	execCtx := executorContextFromEvent(resolvedCtx, e)
	if result.Err != nil {
		execCtx.Err = result.Err
		if result.Event == "" {
			result.Event = EventStepFailed
		}
	}
	if result.Event == "" {
		return
	}
	args := append([]any{execCtx}, result.Args...)
	if err := e.FSM.Event(resolvedCtx, result.Event, args...); err != nil && execCtx.Err == nil {
		execCtx.Err = err
	}
}

type executorObserver struct {
	now     func() time.Time
	baseCtx context.Context
}

func newExecutorObserver(ctx context.Context) *executorObserver {
	return &executorObserver{now: time.Now, baseCtx: ctx}
}

func (o *executorObserver) resolveContext(cbCtx context.Context) context.Context {
	if cbCtx != nil {
		return cbCtx
	}
	if o != nil && o.baseCtx != nil {
		return o.baseCtx
	}
	return context.TODO()
}

func (o *executorObserver) BeforeEvent(cbCtx context.Context, e *fsm.Event) {
	resolvedCtx := o.resolveContext(cbCtx)
	execCtx := executorContextFromEvent(resolvedCtx, e)
	execCtx.transitionStarted = o.now()
	logger.FromContext(resolvedCtx).Debug(
		"Orchestrate FSM transition start",
		"event", e.Event,
		"from_state", e.Src,
		"to_state", e.Dst,
	)
}

func (o *executorObserver) AfterEvent(cbCtx context.Context, e *fsm.Event) {
	resolvedCtx := o.resolveContext(cbCtx)
	execCtx := executorContextFromEvent(resolvedCtx, e)
	duration := time.Duration(0)
	if !execCtx.transitionStarted.IsZero() {
		duration = o.now().Sub(execCtx.transitionStarted)
	}
	fields := []any{"event", e.Event, "from_state", e.Src, "to_state", e.Dst}
	if duration > 0 {
		fields = append(fields, "duration_ms", duration.Milliseconds())
	}
	if execCtx.Err != nil {
		fields = append(fields, "error", core.RedactError(execCtx.Err))
	}
	logger.FromContext(resolvedCtx).Debug("Orchestrate FSM transition complete", fields...)
}

func (o *executorObserver) EnterState(cbCtx context.Context, e *fsm.Event, execCtx *executorContext) {
	resolvedCtx := o.resolveContext(cbCtx)
	logger.FromContext(resolvedCtx).Debug(
		"Orchestrate FSM state entered",
		"state", e.Dst,
		"event", e.Event,
		"step_index", execCtx.StepIndex,
	)
}

func executorContextFromEvent(ctx context.Context, e *fsm.Event) *executorContext {
	if e != nil && len(e.Args) > 0 {
		if execCtx, ok := e.Args[0].(*executorContext); ok && execCtx != nil {
			return execCtx
		}
	}
	logger.FromContext(ctx).Error("Executor FSM context missing", "event", eventName(e))
	return &executorContext{}
}

func eventName(e *fsm.Event) string {
	if e == nil {
		return ""
	}
	return e.Event
}
