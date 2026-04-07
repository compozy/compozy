package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

type jobRunner struct {
	index     int
	job       *job
	execCtx   *jobExecutionContext
	lifecycle *jobLifecycle
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
	if r.execCtx.cfg.dryRun {
		r.lifecycle.markSuccess()
		return
	}

	maxAttempts := atLeastOne(r.execCtx.cfg.maxRetries + 1)
	timeout := r.execCtx.cfg.timeout
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
					codeFile: r.job.codeFileLabel(),
					exitCode: -1,
					outLog:   r.job.outLog,
					errLog:   r.job.errLog,
					err:      err,
				})
				return
			}
			r.lifecycle.markSuccess()
			return
		}
		nextTimeout, continueLoop := r.handleResult(attempt, maxAttempts, timeout, result)
		if !continueLoop {
			return
		}
		timeout = nextTimeout
	}
}

func (r *jobRunner) runPostSuccessHook(ctx context.Context) error {
	return r.execCtx.afterJobSuccess(ctx, r.job)
}

func (r *jobRunner) handleResult(
	attempt int,
	attempts int,
	timeout time.Duration,
	result jobAttemptResult,
) (time.Duration, bool) {
	if result.Successful() {
		r.lifecycle.markSuccess()
		return timeout, false
	}
	if result.IsCanceled() {
		r.lifecycle.markCanceled(result.exitCode)
		return timeout, false
	}
	if !result.NeedsRetry() || attempt == attempts {
		r.lifecycle.markGiveUp(r.ensureFailure(result, "job failed"))
		return timeout, false
	}
	nextTimeout := r.nextTimeout(timeout)
	nextAttempt := attempt + 1
	r.lifecycle.markRetry(r.ensureFailure(result, "retrying job"), nextAttempt, attempts)
	r.logRetry(nextAttempt, attempts, nextTimeout)
	return nextTimeout, true
}

func (r *jobRunner) ensureFailure(result jobAttemptResult, fallback string) failInfo {
	if result.failure != nil {
		return *result.failure
	}
	return failInfo{
		codeFile: r.job.codeFileLabel(),
		exitCode: result.exitCode,
		outLog:   r.job.outLog,
		errLog:   r.job.errLog,
		err:      errors.New(fallback),
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
	next := time.Duration(float64(current) * r.execCtx.cfg.retryBackoffMultiplier)
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
	if !r.execCtx.cfg.humanOutputEnabled() {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"\n🔄 [%s] Job %d (%s) retry attempt %d/%d with timeout %v\n",
		time.Now().Format("15:04:05"),
		r.index+1,
		r.job.codeFileLabel(),
		attempt,
		maxAttempts,
		timeout,
	)
}
