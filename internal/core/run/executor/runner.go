package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/worktree"
)

type jobRunner struct {
	index     int
	job       *job
	execCtx   *jobExecutionContext
	lifecycle *jobLifecycle
	// runAttempt is the seam through which one attempt is dispatched. Production
	// wires it to executeAttempt; tests script attempt outcomes through it.
	runAttempt  func(ctx context.Context, timeout time.Duration) jobAttemptResult
	preSnapshot worktree.Snapshot
}

// attemptBudget tracks the two independent budgets a job draws from. The stall
// budget (StallPolicy.Retries) is deliberately separate from ordinary, so a
// frozen agent is still retried once even when MaxRetries is 0. total is the
// attempt count reported to the UI; it widens only when a stall retry is granted.
type attemptBudget struct {
	ordinary  int
	stall     int
	stallUsed int
	total     int
}

// exhaustedStallReason explains a park caused by an empty stall budget. The two
// cases read very differently in a triage log: one job retried and froze again,
// the other was never allowed a retry at all.
func (b *attemptBudget) exhaustedStallReason() string {
	if b.stallUsed > 0 {
		return "job stalled again after its clean-state retry"
	}
	return "job stalled and no stall retry budget remains"
}

func newJobRunner(index int, jb *job, execCtx *jobExecutionContext) *jobRunner {
	runner := &jobRunner{
		index:     index,
		job:       jb,
		execCtx:   execCtx,
		lifecycle: newJobLifecycle(index, jb, execCtx),
	}
	runner.runAttempt = runner.executeAttempt
	return runner
}

func (r *jobRunner) run(ctx context.Context) {
	r.lifecycle.schedule()
	if r.execCtx.cfg.DryRun {
		r.preSnapshot = r.captureWorkspaceSnapshot(ctx)
	}
	if err := r.dispatchPreExecuteHook(ctx); err != nil {
		r.lifecycle.markGiveUp(failInfo{
			CodeFile: r.job.CodeFileLabel(),
			ExitCode: -1,
			OutLog:   r.job.OutLog,
			ErrLog:   r.job.ErrLog,
			Err:      fmt.Errorf("dispatch job.pre_execute: %w", err),
		})
		return
	}
	defer r.dispatchPostExecuteHook(ctx)
	if r.execCtx.cfg.DryRun {
		r.completeDryRun(ctx)
		return
	}

	r.executeAttempts(ctx)
}

// executeAttempts drives the retry loop until the job reaches a terminal state.
// The loop has no fixed bound: handleResult owns both budgets and decides when
// to stop, so a stall retry can run past the ordinary MaxRetries ceiling.
func (r *jobRunner) executeAttempts(ctx context.Context) {
	r.preSnapshot = r.captureWorkspaceSnapshot(ctx)
	budget := attemptBudget{
		ordinary: r.execCtx.cfg.MaxRetries,
		stall:    r.stallRetries(),
		total:    atLeastOne(r.execCtx.cfg.MaxRetries + 1),
	}
	timeout := r.execCtx.cfg.Timeout
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			r.lifecycle.markCanceled(exitCodeCanceled)
			return
		}

		r.lifecycle.startAttempt(attempt, budget.total, timeout)
		result := r.runAttempt(ctx, timeout)
		if result.Successful() {
			if err := r.runPostSuccessHook(ctx); err != nil {
				r.lifecycle.markGiveUp(failInfo{
					CodeFile: r.job.CodeFileLabel(),
					ExitCode: -1,
					OutLog:   r.job.OutLog,
					ErrLog:   r.job.ErrLog,
					Err:      err,
				})
				return
			}
			r.lifecycle.markSuccess()
			return
		}
		nextTimeout, retryDelay, continueLoop := r.handleResult(ctx, attempt, &budget, timeout, result)
		if !continueLoop {
			return
		}
		if retryDelay > 0 && !r.waitForRetry(ctx, retryDelay) {
			r.lifecycle.markCanceled(exitCodeCanceled)
			return
		}
		timeout = nextTimeout
	}
}

// stallRetries is the one-shot recovery budget for a frozen agent, read from the
// resolved stall policy rather than MaxRetries.
func (r *jobRunner) stallRetries() int {
	cfg := r.execCtx.cfg
	if cfg == nil || !cfg.Stall.Enabled || cfg.Stall.Retries <= 0 {
		return 0
	}
	return cfg.Stall.Retries
}

func (r *jobRunner) completeDryRun(ctx context.Context) {
	if r.execCtx.cfg.Mode == model.ExecutionModePRDTasks {
		_, captured, err := r.execCtx.captureTaskWorktreeScope(ctx, r.job, r.preSnapshot)
		if err == nil && !captured {
			err = errors.New("capture dry-run task worktree scope")
		}
		if err != nil {
			r.lifecycle.markGiveUp(failInfo{
				CodeFile: r.job.CodeFileLabel(),
				ExitCode: -1,
				OutLog:   r.job.OutLog,
				ErrLog:   r.job.ErrLog,
				Err:      err,
			})
			return
		}
	}
	r.lifecycle.markSuccess()
}

func (r *jobRunner) runPostSuccessHook(ctx context.Context) error {
	return r.execCtx.afterJobSuccess(ctx, r.job, r.preSnapshot)
}

// captureWorkspaceSnapshot fingerprints the workspace before the agent is
// dispatched. Two consumers need it: afterTaskJobSuccess compares it against a
// post-run capture to detect agent sessions that ended cleanly without producing
// any code, and the stall retry resets the worktree back to it. Capture's cost is
// skipped entirely when neither consumer can use the result.
func (r *jobRunner) captureWorkspaceSnapshot(ctx context.Context) worktree.Snapshot {
	if r == nil || r.execCtx == nil || r.execCtx.cfg == nil {
		return worktree.Snapshot{}
	}
	if r.execCtx.cfg.Mode != model.ExecutionModePRDTasks && !r.canAttemptCleanReset() {
		return worktree.Snapshot{}
	}
	var (
		snap worktree.Snapshot
		err  error
	)
	if r.execCtx.cfg.Mode == model.ExecutionModePRDTasks {
		snap, err = worktree.CaptureExcluding(
			ctx,
			r.execCtx.cfg.WorkspaceRoot,
			r.execCtx.cfg.TasksDir,
		)
	} else {
		snap, err = worktree.Capture(ctx, r.execCtx.cfg.WorkspaceRoot)
	}
	if err != nil {
		r.execCtx.runtimeLogger().Warn(
			"failed to capture pre-run workspace snapshot; falling back to legacy completion behavior",
			"workspace_root", r.execCtx.cfg.WorkspaceRoot,
			"error", err,
		)
		return worktree.Snapshot{}
	}
	return snap
}

func (r *jobRunner) handleResult(
	ctx context.Context,
	attempt int,
	budget *attemptBudget,
	timeout time.Duration,
	result jobAttemptResult,
) (time.Duration, time.Duration, bool) {
	if result.Successful() {
		r.lifecycle.markSuccess()
		return timeout, 0, false
	}
	if result.IsCanceled() {
		r.lifecycle.markCanceled(result.ExitCode)
		return timeout, 0, false
	}
	if result.IsStalled() {
		return r.handleStall(ctx, attempt, budget, timeout, result)
	}
	if !result.NeedsRetry() || budget.ordinary <= 0 {
		failure := r.ensureFailure(result, "job failed")
		r.lifecycle.markGiveUp(failure)
		r.execCtx.stopJobsAfterAuthenticationFailure(failure.Err)
		return timeout, 0, false
	}
	retryDecision, err := r.dispatchPreRetryHook(ctx, attempt, result)
	if err != nil {
		failure := r.ensureFailure(result, "job failed")
		failure.Err = errors.Join(failure.Err, fmt.Errorf("dispatch job.pre_retry: %w", err))
		r.lifecycle.markGiveUp(failure)
		return timeout, 0, false
	}
	if retryDecision.Proceed != nil && !*retryDecision.Proceed {
		failure := r.ensureFailure(result, "job failed")
		failure.Err = errors.New("retry canceled by extension")
		r.lifecycle.markGiveUp(failure)
		return timeout, 0, false
	}
	budget.ordinary--
	nextTimeout := r.nextTimeout(timeout)
	nextAttempt := attempt + 1
	r.lifecycle.markRetry(r.ensureFailure(result, "retrying job"), nextAttempt, budget.total)
	r.logRetry(nextAttempt, budget.total, nextTimeout)
	return nextTimeout, time.Duration(retryDecision.DelayMS) * time.Millisecond, true
}

// handleStall routes a frozen attempt through the stall recovery path: retry once
// from a clean worktree, park on the second stall. Every exit that is not a retry
// is a park, never a plain failure, so a stalled job always keeps its worktree and
// journal for triage. Timeout is not backed off, because the idle window that
// detects a stall comes from the stall policy, not from the attempt timeout.
func (r *jobRunner) handleStall(
	ctx context.Context,
	attempt int,
	budget *attemptBudget,
	timeout time.Duration,
	result jobAttemptResult,
) (time.Duration, time.Duration, bool) {
	failure := r.ensureFailure(result, "job stalled")
	r.lifecycle.markStalled(failure, result.LastToolCall, budget.total)
	if budget.stall <= 0 {
		r.parkJob(failure, result, budget.exhaustedStallReason())
		return timeout, 0, false
	}
	retryDecision, err := r.dispatchPreRetryHook(ctx, attempt, result)
	if err != nil {
		r.parkJob(failure, result, fmt.Sprintf("dispatch job.pre_retry: %v", err))
		return timeout, 0, false
	}
	if retryDecision.Proceed != nil && !*retryDecision.Proceed {
		r.parkJob(failure, result, "stall retry canceled by extension")
		return timeout, 0, false
	}
	if err := r.resetWorktreeForStallRetry(ctx); err != nil {
		r.parkJob(failure, result, fmt.Sprintf("clean worktree reset is not possible: %v", err))
		return timeout, 0, false
	}
	budget.stall--
	budget.stallUsed++
	// A stall retry grants one attempt beyond the ordinary ceiling, so widen the
	// reported budget by exactly one. Bumping total up to nextAttempt alone
	// under-counts when ordinary retries still follow a stall retry (it would
	// report "3/2"); incrementing keeps total == (MaxRetries+1)+stallUsed on
	// every interleaving of ordinary and stall retries.
	budget.total++
	nextAttempt := attempt + 1
	r.lifecycle.markRetry(failure, nextAttempt, budget.total)
	r.logRetry(nextAttempt, budget.total, timeout)
	return timeout, time.Duration(retryDecision.DelayMS) * time.Millisecond, true
}

func (r *jobRunner) parkJob(failure failInfo, result jobAttemptResult, reason string) {
	failure.Err = fmt.Errorf("%s: %w", reason, failure.Err)
	r.lifecycle.markParked(failure, parkDetail{
		Reason:          reason,
		LastToolCall:    result.LastToolCall,
		LastProgressSeq: r.execCtx.journal.LastSequence(),
		WorktreePath:    r.execCtx.cfg.WorkspaceRoot,
		LogPath:         r.parkedLogPath(),
	})
}

func (r *jobRunner) parkedLogPath() string {
	if path := strings.TrimSpace(r.job.OutLog); path != "" {
		return path
	}
	return strings.TrimSpace(r.job.ErrLog)
}

// canAttemptCleanReset reports whether a clean worktree reset is even conceivable
// for this job. A run with sibling jobs shares one workspace, so resetting it
// would discard a sibling's work; those jobs park on the first stall instead.
func (r *jobRunner) canAttemptCleanReset() bool {
	cfg := r.execCtx.cfg
	return r.stallRetries() > 0 &&
		strings.TrimSpace(cfg.WorkspaceRoot) != "" &&
		r.execCtx.total == 1
}

// resetWorktreeForStallRetry discards everything the stalled attempt produced so
// the retry runs the task fresh and cannot double-apply a side effect. Any error
// means the reset is not possible and the caller must park (ADR-005 fallback).
func (r *jobRunner) resetWorktreeForStallRetry(ctx context.Context) error {
	cfg := r.execCtx.cfg
	root := strings.TrimSpace(cfg.WorkspaceRoot)
	if root == "" {
		return errors.New("workspace root is unknown")
	}
	if r.execCtx.total != 1 {
		return errors.New("workspace is shared with sibling jobs")
	}
	if err := worktree.Reset(ctx, root, r.preSnapshot); err != nil {
		return err
	}
	r.execCtx.runtimeLogger().Info(
		"reset job worktree to a clean state before stall retry",
		"workspace_root", root,
		"code_file", r.job.CodeFileLabel(),
	)
	return nil
}

func (r *jobRunner) ensureFailure(result jobAttemptResult, fallback string) failInfo {
	if result.Failure != nil {
		return *result.Failure
	}
	return failInfo{
		CodeFile: r.job.CodeFileLabel(),
		ExitCode: result.ExitCode,
		OutLog:   r.job.OutLog,
		ErrLog:   r.job.ErrLog,
		Err:      errors.New(fallback),
	}
}

func (r *jobRunner) executeAttempt(ctx context.Context, timeout time.Duration) jobAttemptResult {
	return executeJobWithTimeout(
		ctx,
		r.execCtx.cfg,
		r.job,
		r.execCtx.cwd,
		r.execCtx.ui != nil,
		r.index,
		timeout,
		r.execCtx.journal,
		&r.execCtx.aggregateUsage,
		&r.execCtx.aggregateMu,
		r.execCtx.trackClient,
	)
}

func (r *jobRunner) nextTimeout(current time.Duration) time.Duration {
	if current <= 0 {
		return current
	}
	next := time.Duration(float64(current) * r.execCtx.cfg.RetryBackoffMultiplier)
	const maxTimeout = 30 * time.Minute
	if next > maxTimeout {
		return maxTimeout
	}
	return next
}

func (r *jobRunner) logRetry(attempt int, maxAttempts int, timeout time.Duration) {
	if r.execCtx.ui != nil {
		return
	}
	if !r.execCtx.cfg.HumanOutputEnabled() {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"\n🔄 [%s] Job %d (%s) retry attempt %d/%d with timeout %v\n",
		time.Now().Format("15:04:05"),
		r.index+1,
		r.job.CodeFileLabel(),
		attempt,
		maxAttempts,
		timeout,
	)
}

func (r *jobRunner) dispatchPreExecuteHook(ctx context.Context) error {
	if r == nil || r.execCtx == nil || r.execCtx.cfg == nil {
		return nil
	}

	before := hookModelJob(r.job)
	payload, err := model.DispatchMutableHook(
		ctx,
		r.execCtx.cfg.RuntimeManager,
		"job.pre_execute",
		jobPreExecutePayload{
			RunID: r.execCtx.cfg.RunArtifacts.RunID,
			Job:   hookModelJob(r.job),
		},
	)
	if err != nil {
		return err
	}
	if jobRuntimeChanged(before, payload.Job) {
		return fmt.Errorf("job.pre_execute cannot mutate job runtime after planning completed")
	}
	applyHookModelJob(r.job, payload.Job)
	return nil
}

func (r *jobRunner) dispatchPostExecuteHook(ctx context.Context) {
	if r == nil || r.execCtx == nil || r.execCtx.cfg == nil {
		return
	}

	model.DispatchObserverHook(
		ctx,
		r.execCtx.cfg.RuntimeManager,
		"job.post_execute",
		jobPostExecutePayload{
			RunID:  r.execCtx.cfg.RunArtifacts.RunID,
			Job:    hookModelJob(r.job),
			Result: r.hookJobResult(),
		},
	)
}

func (r *jobRunner) dispatchPreRetryHook(
	ctx context.Context,
	attempt int,
	result jobAttemptResult,
) (jobPreRetryPayload, error) {
	if r == nil || r.execCtx == nil || r.execCtx.cfg == nil {
		return jobPreRetryPayload{}, nil
	}

	failure := r.ensureFailure(result, "job failed")
	payload, err := model.DispatchMutableHook(
		ctx,
		r.execCtx.cfg.RuntimeManager,
		"job.pre_retry",
		jobPreRetryPayload{
			RunID:     r.execCtx.cfg.RunArtifacts.RunID,
			Job:       hookModelJob(r.job),
			Attempt:   attempt,
			LastError: failure.Err.Error(),
		},
	)
	if err != nil {
		return jobPreRetryPayload{}, err
	}
	return payload, nil
}

func (r *jobRunner) waitForRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func jobRuntimeChanged(before model.Job, after model.Job) bool {
	return before.IDE != after.IDE ||
		before.Model != after.Model ||
		before.ReasoningEffort != after.ReasoningEffort
}
