package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/worktree"
)

type jobRunner struct {
	index       int
	job         *job
	execCtx     *jobExecutionContext
	lifecycle   *jobLifecycle
	preSnapshot worktree.Snapshot
}

func newJobRunner(index int, jb *job, execCtx *jobExecutionContext) *jobRunner {
	return &jobRunner{
		index:     index,
		job:       jb,
		execCtx:   execCtx,
		lifecycle: newJobLifecycle(index, jb, execCtx),
	}
}

func (r *jobRunner) run(ctx context.Context) {
	r.lifecycle.schedule()
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
		r.lifecycle.markSuccess()
		return
	}

	r.preSnapshot = r.captureWorkspaceSnapshot(ctx)

	maxAttempts := atLeastOne(r.execCtx.cfg.MaxRetries + 1)
	timeout := r.execCtx.cfg.Timeout
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			r.lifecycle.markCanceled(exitCodeCanceled)
			return
		}

		r.lifecycle.startAttempt(attempt, maxAttempts, timeout)
		result := r.executeAttempt(ctx, timeout)
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
		nextTimeout, retryDelay, continueLoop := r.handleResult(ctx, attempt, maxAttempts, timeout, result)
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

func (r *jobRunner) runPostSuccessHook(ctx context.Context) error {
	return r.execCtx.afterJobSuccess(ctx, r.job, r.preSnapshot)
}

// captureWorkspaceSnapshot fingerprints the workspace before the agent is
// dispatched so afterTaskJobSuccess can compare against a post-run capture and
// detect agent sessions that ended cleanly without producing any code. Only
// PRD-tasks mode currently consumes the snapshot; in other modes Capture's
// cost is paid once but the result is unused.
func (r *jobRunner) captureWorkspaceSnapshot(ctx context.Context) worktree.Snapshot {
	if r == nil || r.execCtx == nil || r.execCtx.cfg == nil {
		return worktree.Snapshot{}
	}
	if r.execCtx.cfg.Mode != model.ExecutionModePRDTasks {
		return worktree.Snapshot{}
	}
	snap, err := worktree.Capture(ctx, r.execCtx.cfg.WorkspaceRoot)
	if err != nil {
		r.execCtx.logger.Warn(
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
	attempts int,
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
	if !result.NeedsRetry() || attempt == attempts {
		r.lifecycle.markGiveUp(r.ensureFailure(result, "job failed"))
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
	nextTimeout := r.nextTimeout(timeout)
	nextAttempt := attempt + 1
	r.lifecycle.markRetry(r.ensureFailure(result, "retrying job"), nextAttempt, attempts)
	r.logRetry(nextAttempt, attempts, nextTimeout)
	return nextTimeout, time.Duration(retryDecision.DelayMS) * time.Millisecond, true
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
