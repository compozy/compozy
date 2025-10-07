package orchestrate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	agentexec "github.com/compozy/compozy/engine/agent/exec"
	"github.com/compozy/compozy/engine/core"
	toolcontext "github.com/compozy/compozy/engine/tool/context"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/sync/errgroup"
)

const defaultMaxParallel = 4

var (
	errRunnerRequired   = errors.New("agent orchestrator runner is required")
	errPlanMissing      = errors.New("agent orchestrator plan is required")
	errMaxDepthExceeded = errors.New("agent orchestrator maximum depth exceeded")
	errMaxStepsExceeded = errors.New("agent orchestrator maximum steps exceeded")
	errUnknownStepType  = errors.New("agent orchestrator encountered unknown step type")
	errParallelNoSteps  = errors.New("agent orchestrator parallel step requires at least one child")
)

type Runner interface {
	Execute(context.Context, agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error)
}

type Limits struct {
	MaxDepth       int
	MaxSteps       int
	MaxParallel    int
	DefaultTimeout time.Duration
}

type Engine struct {
	runner Runner
	limits Limits
	now    func() time.Time
}

type StepResult struct {
	StepID   string
	Type     StepType
	Status   StepStatus
	ExecID   core.ID
	Output   *core.Output
	Error    error
	Children []StepResult
	Elapsed  time.Duration
}

type executorContext struct {
	Plan              *Plan
	Bindings          map[string]any
	Results           []StepResult
	StepIndex         int
	CurrentStep       *Step
	PendingResult     *StepResult
	Err               error
	startedAt         time.Time
	transitionStarted time.Time
}

func NewEngine(runner Runner, limits Limits) *Engine {
	engine := &Engine{runner: runner, limits: limits, now: time.Now}
	if engine.limits.MaxParallel <= 0 {
		engine.limits.MaxParallel = defaultMaxParallel
	}
	return engine
}

func (e *Engine) Run(ctx context.Context, plan *Plan) ([]StepResult, error) {
	if e == nil || e.runner == nil {
		return nil, errRunnerRequired
	}
	if plan == nil {
		return nil, errPlanMissing
	}
	log := logger.FromContext(ctx)
	_ = config.FromContext(ctx)
	depth := toolcontext.AgentOrchestratorDepth(ctx)
	if e.limits.MaxDepth > 0 && depth >= e.limits.MaxDepth {
		return nil, errMaxDepthExceeded
	}
	if e.limits.MaxSteps > 0 && len(plan.Steps) > e.limits.MaxSteps {
		return nil, errMaxStepsExceeded
	}
	execCtx := &executorContext{
		Plan:     plan,
		Bindings: cloneBindings(plan.Bindings),
		Results:  make([]StepResult, 0, len(plan.Steps)),
	}
	baseCtx := toolcontext.IncrementAgentOrchestratorDepth(ctx)
	fsm := newExecutorFSM(baseCtx, e, execCtx)
	if err := fsm.Event(baseCtx, EventStartPlan, execCtx); err != nil {
		e.ensureFailureResult(execCtx, plan, err)
		if execCtx.Err != nil {
			return execCtx.Results, execCtx.Err
		}
		return execCtx.Results, err
	}
	if execCtx.Err != nil && len(execCtx.Results) == 0 {
		e.ensureFailureResult(execCtx, plan, execCtx.Err)
	}
	switch fsm.Current() {
	case StateCompleted:
		log.Debug("Agent orchestrator executor completed", "steps", len(execCtx.Results))
		return execCtx.Results, nil
	case StateFailed:
		e.ensureFailureResult(execCtx, plan, execCtx.Err)
		if execCtx.Err != nil {
			return execCtx.Results, execCtx.Err
		}
		return execCtx.Results, fmt.Errorf("agent orchestrator executor failed")
	default:
		return execCtx.Results, fmt.Errorf("agent orchestrator executor finished in unexpected state %s", fsm.Current())
	}
}

func (e *Engine) OnEnterPending(context.Context, *executorContext) TransitionResult {
	return TransitionResult{}
}

func (e *Engine) OnEnterPlanning(_ context.Context, execCtx *executorContext) TransitionResult {
	if execCtx == nil || execCtx.Plan == nil {
		execCtx.Err = errPlanMissing
		return TransitionResult{Event: EventValidationFailed, Err: errPlanMissing}
	}
	if err := execCtx.Plan.Validate(); err != nil {
		execCtx.Err = err
		return TransitionResult{Event: EventValidationFailed, Err: err}
	}
	return TransitionResult{Event: EventPlannerFinished, Args: []any{execCtx}}
}

func (e *Engine) OnEnterDispatching(_ context.Context, execCtx *executorContext) TransitionResult {
	if execCtx.StepIndex >= len(execCtx.Plan.Steps) {
		return TransitionResult{Event: EventParallelComplete, Args: []any{execCtx}}
	}
	execCtx.CurrentStep = &execCtx.Plan.Steps[execCtx.StepIndex]
	execCtx.startedAt = e.now()
	return TransitionResult{Event: EventDispatchStep, Args: []any{execCtx}}
}

func (e *Engine) OnEnterAwaitingResults(ctx context.Context, execCtx *executorContext) TransitionResult {
	if execCtx.CurrentStep == nil {
		execCtx.Err = errUnknownStepType
		return TransitionResult{Event: EventStepFailed, Err: errUnknownStepType}
	}
	result, err := e.executeStep(ctx, execCtx.CurrentStep)
	execCtx.PendingResult = &result
	if err != nil {
		execCtx.Err = err
		return TransitionResult{Event: EventStepFailed, Err: err}
	}
	if execCtx.CurrentStep.Type == StepTypeParallel {
		return TransitionResult{Event: EventParallelComplete, Args: []any{execCtx}}
	}
	return TransitionResult{Event: EventStepSucceeded, Args: []any{execCtx}}
}

func (e *Engine) OnEnterMerging(_ context.Context, execCtx *executorContext) TransitionResult {
	if execCtx.PendingResult != nil {
		execCtx.PendingResult.Elapsed = e.now().Sub(execCtx.startedAt)
		execCtx.Results = append(execCtx.Results, *execCtx.PendingResult)
		e.applyBindings(execCtx, execCtx.PendingResult)
	}
	execCtx.StepIndex++
	execCtx.PendingResult = nil
	execCtx.CurrentStep = nil
	if execCtx.StepIndex >= len(execCtx.Plan.Steps) {
		return TransitionResult{Event: EventParallelComplete, Args: []any{execCtx}}
	}
	return TransitionResult{Event: EventPlannerFinished, Args: []any{execCtx}}
}

func (e *Engine) OnEnterCompleted(context.Context, *executorContext) TransitionResult {
	return TransitionResult{}
}

func (e *Engine) OnEnterFailed(_ context.Context, execCtx *executorContext) TransitionResult {
	if execCtx.PendingResult != nil {
		execCtx.PendingResult.Elapsed = e.now().Sub(execCtx.startedAt)
		execCtx.Results = append(execCtx.Results, *execCtx.PendingResult)
		execCtx.PendingResult = nil
	}
	execCtx.CurrentStep = nil
	return TransitionResult{}
}

func (e *Engine) OnFailure(ctx context.Context, execCtx *executorContext, event string) {
	logger.FromContext(ctx).Warn("Agent orchestrator executor transition failed", "event", event, "error", execCtx.Err)
}

func (e *Engine) executeStep(ctx context.Context, step *Step) (StepResult, error) {
	switch step.Type {
	case StepTypeAgent:
		return e.executeAgentStep(ctx, step.ID, step.Agent)
	case StepTypeParallel:
		return e.executeParallelStep(ctx, step)
	default:
		return StepResult{StepID: step.ID, Type: step.Type, Status: StepStatusFailed}, errUnknownStepType
	}
}

func (e *Engine) executeAgentStep(ctx context.Context, stepID string, agentStep *AgentStep) (StepResult, error) {
	if agentStep == nil {
		return StepResult{StepID: stepID, Type: StepTypeAgent, Status: StepStatusFailed}, errUnknownStepType
	}
	timeout := e.stepTimeout(ctx, agentStep.TimeoutMs)
	stepCtx, cancel := e.withTimeout(ctx, timeout)
	if cancel != nil {
		defer cancel()
	}
	req := agentexec.ExecuteRequest{
		AgentID: agentStep.AgentID,
		Action:  agentStep.ActionID,
		Prompt:  agentStep.Prompt,
		With:    core.NewInput(agentStep.With),
		Timeout: timeout,
	}
	res, err := e.runner.Execute(stepCtx, req)
	if err != nil {
		return StepResult{StepID: stepID, Type: StepTypeAgent, Status: StepStatusFailed, Error: err}, err
	}
	return StepResult{
		StepID: stepID,
		Type:   StepTypeAgent,
		Status: StepStatusSuccess,
		ExecID: res.ExecID,
		Output: res.Output,
	}, nil
}

func (e *Engine) executeParallelStep(ctx context.Context, step *Step) (StepResult, error) {
	parallel := step.Parallel
	if parallel == nil || len(parallel.Steps) == 0 {
		return StepResult{StepID: step.ID, Type: StepTypeParallel, Status: StepStatusFailed}, errParallelNoSteps
	}
	limit := e.parallelLimit(len(parallel.Steps), parallel.MaxConcurrency)
	childResults := make([]StepResult, len(parallel.Steps))
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(limit)
	for idx := range parallel.Steps {
		idx := idx
		childStep := &parallel.Steps[idx]
		childID := fmt.Sprintf("%s[%d]", step.ID, idx)
		group.Go(func() error {
			result, err := e.executeAgentStep(groupCtx, childID, childStep)
			mu.Lock()
			childResults[idx] = result
			mu.Unlock()
			return err
		})
	}
	err := group.Wait()
	status := summarizeParallelStatus(childResults)
	return StepResult{StepID: step.ID, Type: StepTypeParallel, Status: status, Children: childResults}, err
}

func (e *Engine) applyBindings(execCtx *executorContext, result *StepResult) {
	if result == nil {
		return
	}
	switch result.Type {
	case StepTypeAgent:
		e.storeAgentBinding(execCtx, result)
	case StepTypeParallel:
		e.storeParallelBinding(execCtx, result)
	}
}

func (e *Engine) storeAgentBinding(execCtx *executorContext, result *StepResult) {
	step := execCtx.CurrentStep
	if step == nil || step.Agent == nil || step.Agent.ResultKey == "" || result.Output == nil {
		return
	}
	execCtx.Bindings[step.Agent.ResultKey] = result.Output.AsMap()
}

func (e *Engine) storeParallelBinding(execCtx *executorContext, result *StepResult) {
	step := execCtx.CurrentStep
	if step == nil || step.Parallel == nil {
		return
	}
	aggregated := make(map[string]any)
	for idx := range step.Parallel.Steps {
		childStep := step.Parallel.Steps[idx]
		childResult := result.Children[idx]
		if childStep.ResultKey != "" && childResult.Output != nil {
			value := childResult.Output.AsMap()
			execCtx.Bindings[childStep.ResultKey] = value
			aggregated[childStep.ResultKey] = value
		}
	}
	if step.Parallel.ResultKey != "" {
		execCtx.Bindings[step.Parallel.ResultKey] = aggregated
	}
}

func (e *Engine) stepTimeout(ctx context.Context, overrideMs int) time.Duration {
	if overrideMs > 0 {
		return e.limitByDeadline(ctx, time.Duration(overrideMs)*time.Millisecond)
	}
	if e.limits.DefaultTimeout > 0 {
		return e.limitByDeadline(ctx, e.limits.DefaultTimeout)
	}
	return e.limitByDeadline(ctx, 0)
}

func (e *Engine) withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, timeout)
}

func (e *Engine) limitByDeadline(ctx context.Context, timeout time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return timeout
	}
	remaining := time.Until(deadline)
	if timeout == 0 || remaining < timeout {
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return timeout
}

func (e *Engine) parallelLimit(children int, requested int) int {
	limit := e.limits.MaxParallel
	if limit <= 0 || limit > children {
		limit = children
	}
	if requested > 0 && requested < limit {
		return requested
	}
	return limit
}

func summarizeParallelStatus(results []StepResult) StepStatus {
	successes := 0
	failures := 0
	for idx := range results {
		switch results[idx].Status {
		case StepStatusSuccess:
			successes++
		case StepStatusFailed:
			failures++
		default:
			failures++
		}
	}
	switch {
	case failures == 0:
		return StepStatusSuccess
	case successes == 0:
		return StepStatusFailed
	default:
		return StepStatusPartial
	}
}

func cloneBindings(src map[string]any) map[string]any {
	if len(src) == 0 {
		return make(map[string]any)
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}

func (e *Engine) ensureFailureResult(execCtx *executorContext, plan *Plan, failureErr error) {
	if execCtx == nil {
		return
	}
	if execCtx.PendingResult != nil {
		if execCtx.PendingResult.Error == nil {
			execCtx.PendingResult.Error = failureErr
		}
		execCtx.PendingResult.Elapsed = e.now().Sub(execCtx.startedAt)
		execCtx.Results = append(execCtx.Results, *execCtx.PendingResult)
		execCtx.PendingResult = nil
		return
	}
	if failureErr == nil {
		failureErr = execCtx.Err
	}
	if failureErr == nil {
		return
	}
	var step *Step
	if execCtx.CurrentStep != nil {
		step = execCtx.CurrentStep
	} else if plan != nil {
		if execCtx.StepIndex < len(plan.Steps) {
			step = &plan.Steps[execCtx.StepIndex]
		} else if len(plan.Steps) > 0 {
			step = &plan.Steps[len(plan.Steps)-1]
		}
	}
	failure := StepResult{Status: StepStatusFailed, Error: failureErr}
	if step != nil {
		failure.StepID = step.ID
		failure.Type = step.Type
	}
	execCtx.Results = append(execCtx.Results, failure)
}
