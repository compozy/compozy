package workflowexec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
)

const (
	workflowPollInitialBackoff = 200 * time.Millisecond
	workflowPollMaxBackoff     = 5 * time.Second
	defaultWorkflowTimeout     = 5 * time.Minute
)

var (
	ErrWorkflowIDRequired      = errors.New("workflow id is required")
	ErrStateRequired           = errors.New("application state is required")
	ErrRepositoryRequired      = errors.New("workflow repository is required")
	ErrWorkerUnavailable       = errors.New("workflow worker is unavailable")
	ErrNegativeWorkflowTimeout = errors.New("timeout must be non-negative")
)

// Runner executes workflows synchronously using the worker trigger pipeline.
type Runner struct {
	state  *appstate.State
	repo   workflow.Repository
	worker *worker.Worker
}

// ExecuteRequest describes a workflow execution request.
type ExecuteRequest struct {
	WorkflowID    string
	Input         core.Input
	InitialTaskID string
	Timeout       time.Duration
}

// ExecuteResult represents the outcome of a synchronous workflow execution.
type ExecuteResult struct {
	ExecID core.ID
	Status core.StatusType
	Output *core.Output
}

// PreparedExecution bundles resolved execution dependencies.
type PreparedExecution struct {
	Request ExecuteRequest
	Timeout time.Duration
}

// NewRunner constructs a Runner instance with the provided dependencies.
func NewRunner(state *appstate.State, repo workflow.Repository, worker *worker.Worker) *Runner {
	return &Runner{state: state, repo: repo, worker: worker}
}

// ExecuteWorkflow satisfies toolenv.WorkflowExecutor.
func (r *Runner) ExecuteWorkflow(ctx context.Context, req toolenv.WorkflowRequest) (*toolenv.WorkflowResult, error) {
	execReq := ExecuteRequest{
		WorkflowID:    req.WorkflowID,
		Input:         req.Input,
		InitialTaskID: req.InitialTaskID,
		Timeout:       req.Timeout,
	}
	result, err := r.Execute(ctx, execReq)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &toolenv.WorkflowResult{
		WorkflowExecID: result.ExecID,
		Output:         result.Output,
		Status:         string(result.Status),
	}, nil
}

// Execute runs a workflow synchronously, waiting for completion or timeout.
func (r *Runner) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	prepared, err := r.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}
	return r.ExecutePrepared(ctx, prepared)
}

// Prepare validates inputs and resolves configuration defaults.
func (r *Runner) Prepare(ctx context.Context, req ExecuteRequest) (*PreparedExecution, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if r.state == nil {
		return nil, ErrStateRequired
	}
	if r.repo == nil {
		return nil, ErrRepositoryRequired
	}
	if req.WorkflowID == "" {
		return nil, ErrWorkflowIDRequired
	}
	timeout, err := resolveWorkflowTimeout(ctx, req.Timeout)
	if err != nil {
		return nil, err
	}
	return &PreparedExecution{Request: req, Timeout: timeout}, nil
}

// ExecutePrepared triggers the workflow and waits for completion.
func (r *Runner) ExecutePrepared(ctx context.Context, prepared *PreparedExecution) (*ExecuteResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if prepared == nil {
		return nil, fmt.Errorf("prepared execution is required")
	}
	worker, err := r.resolveWorker()
	if err != nil {
		return nil, err
	}
	inputPtr := workflowInputPointer(prepared.Request.Input)
	triggered, err := worker.TriggerWorkflow(ctx, prepared.Request.WorkflowID, inputPtr, prepared.Request.InitialTaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger workflow %s: %w", prepared.Request.WorkflowID, err)
	}
	state, timedOut, pollErr := waitForWorkflowCompletion(ctx, r.repo, triggered.WorkflowExecID, prepared.Timeout)
	if pollErr != nil {
		return nil, fmt.Errorf("failed to monitor workflow %s: %w", prepared.Request.WorkflowID, pollErr)
	}
	status := core.StatusTimedOut
	if state != nil {
		status = state.Status
	} else if !timedOut {
		status = core.StatusSuccess
	}
	if timedOut {
		status = core.StatusTimedOut
	}
	var output *core.Output
	if state != nil && state.Output != nil {
		if clone, err := state.Output.Clone(); err == nil && clone != nil {
			output = clone
		} else if copied, err := core.DeepCopyOutput(*state.Output, core.Output{}); err == nil && copied != nil {
			output = &copied
		} else {
			fallback := core.Output(core.CloneMap(map[string]any(*state.Output)))
			output = &fallback
		}
	}
	return &ExecuteResult{
		ExecID: triggered.WorkflowExecID,
		Status: status,
		Output: output,
	}, nil
}

func (r *Runner) resolveWorker() (*worker.Worker, error) {
	if r.worker != nil {
		return r.worker, nil
	}
	if r.state != nil && r.state.Worker != nil {
		return r.state.Worker, nil
	}
	return nil, ErrWorkerUnavailable
}

func workflowInputPointer(input core.Input) *core.Input {
	if input == nil {
		return nil
	}
	clone, err := core.DeepCopyInput(input, core.Input{})
	if err == nil {
		return &clone
	}
	shallow := core.Input(core.CloneMap(map[string]any(input)))
	return &shallow
}

func resolveWorkflowTimeout(ctx context.Context, requested time.Duration) (time.Duration, error) {
	if requested < 0 {
		return 0, ErrNegativeWorkflowTimeout
	}
	defaults := config.DefaultNativeToolsConfig()
	workflowCfg := defaults.CallWorkflow
	if appCfg := config.FromContext(ctx); appCfg != nil {
		workflowCfg = appCfg.Runtime.NativeTools.CallWorkflow
	}
	timeout := workflowCfg.DefaultTimeout
	if timeout <= 0 {
		timeout = defaultWorkflowTimeout
	}
	if requested > 0 {
		timeout = requested
	}
	return timeout, nil
}

func waitForWorkflowCompletion(
	ctx context.Context,
	repo workflow.Repository,
	execID core.ID,
	deadline time.Duration,
) (*workflow.State, bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	interval := workflowPollInitialBackoff
	if interval <= 0 {
		interval = time.Millisecond
	}
	timer := time.NewTimer(0)
	<-timer.C
	defer timer.Stop()
	attempt := 0
	var lastState *workflow.State
	for {
		if pollCtx.Err() != nil {
			state, err := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, err
		}
		state, err := repo.GetState(pollCtx, execID)
		if err != nil {
			if !isIgnorablePollError(err) {
				return lastState, false, err
			}
		} else if state != nil {
			lastState = state
			if isWorkflowTerminal(state.Status) {
				return state, false, nil
			}
		}
		wait := applyWorkflowJitter(interval, execID.String(), attempt)
		interval = nextWorkflowBackoff(interval)
		if !waitForNextWorkflowPoll(pollCtx, timer, wait) {
			state, err := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, err
		}
		attempt++
	}
}

func isWorkflowTerminal(status core.StatusType) bool {
	switch status {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}

func isIgnorablePollError(err error) bool {
	return errors.Is(err, store.ErrWorkflowNotFound) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func applyWorkflowJitter(base time.Duration, execID string, attempt int) time.Duration {
	if base <= 0 {
		return time.Millisecond
	}
	span := base / 10
	if span <= 0 {
		span = time.Millisecond
	}
	spanNanos := int64(span)
	rangeSize := spanNanos*2 + 1
	if rangeSize <= 0 {
		rangeSize = 1
	}
	hashVal := computeJitterHash(execID, attempt, rangeSize)
	offset := hashVal - spanNanos
	result := base + time.Duration(offset)
	if result < time.Millisecond {
		return time.Millisecond
	}
	return result
}

func computeJitterHash(execID string, attempt int, rangeSize int64) int64 {
	hashVal := int64(5381)
	for i := 0; i < len(execID); i++ {
		hashVal = djb2Step(hashVal, int64(execID[i]), rangeSize)
	}
	if attempt < 0 {
		hashVal = djb2Step(hashVal, int64('-'), rangeSize)
	} else {
		hashVal = djb2Step(hashVal, int64('+'), rangeSize)
	}
	for _, d := range formatAttemptDigits(attempt) {
		hashVal = djb2Step(hashVal, int64(d), rangeSize)
	}
	return hashVal % rangeSize
}

func djb2Step(hash, value, mod int64) int64 {
	return ((hash << 5) + hash + value) % mod
}

func formatAttemptDigits(attempt int) []byte {
	value := attempt
	if value < 0 {
		value = -value
	}
	if value == 0 {
		return []byte{'0'}
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append([]byte{'0' + byte(value%10)}, digits...)
		value /= 10
	}
	return digits
}

func nextWorkflowBackoff(current time.Duration) time.Duration {
	if current >= workflowPollMaxBackoff {
		return workflowPollMaxBackoff
	}
	next := current * 2
	if next > workflowPollMaxBackoff {
		return workflowPollMaxBackoff
	}
	return next
}

func waitForNextWorkflowPoll(ctx context.Context, timer *time.Timer, delay time.Duration) bool {
	if delay <= 0 {
		delay = time.Millisecond
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return false
	case <-timer.C:
		return true
	}
}

func finalWorkflowState(
	ctx context.Context,
	repo workflow.Repository,
	execID core.ID,
	lastState *workflow.State,
) (*workflow.State, error) {
	state, err := repo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil {
		if errors.Is(err, store.ErrWorkflowNotFound) {
			return lastState, nil
		}
		return lastState, err
	}
	if state == nil {
		return lastState, nil
	}
	return state, nil
}
