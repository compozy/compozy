package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
)

func executeJobWithTimeout(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	index int,
	timeout time.Duration,
	runJournal *journal.Journal,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	trackClient func(agent.Client) func(),
) jobAttemptResult {
	emitHuman := cfg.humanOutputEnabled() && !useUI
	attemptCtx := ctx
	cancel := func(error) {}
	stopActivityWatchdog := func() {}
	var activity *activityMonitor
	if timeout > 0 {
		activity = newActivityMonitor()
		attemptCtx, cancel = context.WithCancelCause(ctx)
		stopActivityWatchdog = startACPActivityWatchdog(attemptCtx, activity, timeout, cancel)
	}
	defer func() {
		stopActivityWatchdog()
		cancel(nil)
	}()

	execution, err := setupSessionExecution(
		attemptCtx,
		cfg,
		j,
		cwd,
		useUI,
		cfg.humanOutputEnabled(),
		index,
		runJournal,
		aggregateUsage,
		aggregateMu,
		activity,
		runtimeLoggerFor(cfg, useUI),
		trackClient,
	)
	if err != nil {
		if timeout > 0 && isActivityTimeout(err) {
			return handleSessionTimeout(resolveTimeoutError(timeout, err), j, index, emitHuman, timeout)
		}
		fail := recordFailureWithContext(nil, j, nil, err, -1)
		return jobAttemptResult{
			status:    attemptStatusSetupFailed,
			exitCode:  -1,
			failure:   &fail,
			retryable: retryableSetupFailure(err),
		}
	}
	return executeSessionAndResolve(attemptCtx, timeout, execution, j, index, emitHuman)
}

type activityTimeoutError struct {
	timeout time.Duration
}

func (e *activityTimeoutError) Error() string {
	return fmt.Sprintf("activity timeout: no output received for %v", e.timeout)
}

func startACPActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	cancel context.CancelCauseFunc,
) func() {
	if monitor == nil || timeout <= 0 || cancel == nil {
		return func() {}
	}

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	interval := timeout / 2
	if interval <= 0 || interval > activityCheckInterval {
		interval = activityCheckInterval
	}
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if monitor.timeSinceLastActivity() > timeout {
					cancel(&activityTimeoutError{timeout: timeout})
					return
				}
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(stopCh)
		})
	}
}

func createLogFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func handleNilExecution(j *job, index int, emitHuman bool) jobAttemptResult {
	codeFileLabel := j.codeFileLabel()
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: -1,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      fmt.Errorf("failed to set up ACP session execution"),
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Failed to set up job %d (%s): %v\n", index+1, codeFileLabel, failure.err)
	}
	return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failure}
}

func executeSessionAndResolve(
	ctx context.Context,
	timeout time.Duration,
	execution *sessionExecution,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if execution == nil || execution.session == nil {
		return handleNilExecution(j, index, emitHuman)
	}
	defer execution.close()

	streamErrCh := make(chan error, 1)
	go func() {
		streamErrCh <- streamSessionUpdates(execution.session, execution.handler)
	}()

	select {
	case <-execution.session.Done():
		streamErr := <-streamErrCh
		if streamErr != nil {
			if err := execution.handler.HandleCompletion(streamErr); err != nil {
				execution.logger.Warn("failed to finalize ACP session handler after stream error", "error", err)
			}
			appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + streamErr.Error()})
			return buildFailureResult(streamErr, -1, j, index, emitHuman)
		}

		sessionErr := execution.session.Err()
		if err := execution.handler.HandleCompletion(sessionErr); err != nil {
			execution.logger.Warn("failed to finalize ACP session handler", "error", err)
		}
		if sessionErr != nil {
			appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + sessionErr.Error()})
		}
		return handleSessionCompletion(ctx, sessionErr, timeout, j, index, emitHuman)
	case <-ctx.Done():
		cancelErr := context.Cause(ctx)
		if cancelErr == nil {
			cancelErr = ctx.Err()
		}
		if err := execution.handler.HandleCompletion(cancelErr); err != nil {
			execution.logger.Warn("failed to finalize ACP session handler after context cancellation", "error", err)
		}
		appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + cancelErr.Error()})
		if isSessionTimeout(ctx, cancelErr) {
			return handleSessionTimeout(
				resolveTimeoutError(timeout, cancelErr, context.Cause(ctx), ctx.Err()),
				j,
				index,
				emitHuman,
				timeout,
			)
		}
		return handleSessionCancellation(cancelErr, j, index, emitHuman)
	}
}

func streamSessionUpdates(session agent.Session, handler *sessionUpdateHandler) error {
	for update := range session.Updates() {
		if err := handler.HandleUpdate(update); err != nil {
			return err
		}
	}
	return nil
}

func handleSessionCompletion(
	ctx context.Context,
	sessionErr error,
	timeout time.Duration,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if sessionErr == nil {
		return jobAttemptResult{status: attemptStatusSuccess, exitCode: 0}
	}

	if isSessionTimeout(ctx, sessionErr) {
		return handleSessionTimeout(
			resolveTimeoutError(timeout, sessionErr, context.Cause(ctx), ctx.Err()),
			j,
			index,
			emitHuman,
			timeout,
		)
	}
	if errors.Is(sessionErr, context.Canceled) {
		return handleSessionCancellation(sessionErr, j, index, emitHuman)
	}

	exitCode := sessionErrorCode(sessionErr)
	return buildFailureResult(sessionErr, exitCode, j, index, emitHuman)
}

func isSessionTimeout(ctx context.Context, err error) bool {
	return isActivityTimeout(err) ||
		isActivityTimeout(context.Cause(ctx)) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func isActivityTimeout(err error) bool {
	var timeoutErr *activityTimeoutError
	return errors.As(err, &timeoutErr)
}

func resolveTimeoutError(timeout time.Duration, errs ...error) error {
	for _, err := range errs {
		var timeoutErr *activityTimeoutError
		if errors.As(err, &timeoutErr) {
			return timeoutErr
		}
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return &activityTimeoutError{timeout: timeout}
}

func handleSessionCancellation(
	cancelErr error,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nCanceling job %d (%s) due to shutdown signal\n",
			index+1,
			j.codeFileLabel(),
		)
	}
	codeFileLabel := j.codeFileLabel()
	if cancelErr == nil {
		cancelErr = fmt.Errorf("job canceled by shutdown")
	}
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeCanceled,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      cancelErr,
	}
	return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failure}
}

func handleSessionTimeout(
	timeoutErr error,
	j *job,
	index int,
	emitHuman bool,
	timeout time.Duration,
) jobAttemptResult {
	logTimeoutMessage(index, j, timeout, emitHuman)
	codeFileLabel := j.codeFileLabel()
	if timeoutErr == nil {
		timeoutErr = fmt.Errorf("ACP session timeout after %v", timeout)
	}
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeTimeout,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      timeoutErr,
	}
	return jobAttemptResult{
		status:    attemptStatusTimeout,
		exitCode:  exitCodeTimeout,
		failure:   &failure,
		retryable: true,
	}
}

func logTimeoutMessage(index int, j *job, timeout time.Duration, emitHuman bool) {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nJob %d (%s) timed out after %v of inactivity\n",
			index+1,
			j.codeFileLabel(),
			timeout,
		)
	}
}

func buildFailureResult(err error, exitCode int, j *job, index int, emitHuman bool) jobAttemptResult {
	codeFileLabel := j.codeFileLabel()
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      err,
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Job %d (%s) failed with code %d: %v\n", index+1, codeFileLabel, exitCode, err)
	}
	return jobAttemptResult{
		status:    attemptStatusFailure,
		exitCode:  exitCode,
		failure:   &failure,
		retryable: true,
	}
}

func retryableSetupFailure(err error) bool {
	var setupErr *agent.SessionSetupError
	if !errors.As(err, &setupErr) {
		return false
	}

	switch setupErr.Stage {
	case agent.SessionSetupStageStartProcess, agent.SessionSetupStageInitialize, agent.SessionSetupStageNewSession:
		return true
	default:
		return false
	}
}

func sessionErrorCode(err error) int {
	var sessionErr *agent.SessionError
	if errors.As(err, &sessionErr) {
		return sessionErr.Code
	}
	return -1
}

func recordFailureWithContext(
	failuresMu *sync.Mutex,
	j *job,
	failures *[]failInfo,
	err error,
	exitCode int,
) failInfo {
	codeFileLabel := j.codeFileLabel()
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      err,
	}
	recordFailure(failuresMu, failures, failure)
	return failure
}

func recordFailure(mu *sync.Mutex, list *[]failInfo, f failInfo) {
	if list == nil {
		return
	}
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	*list = append(*list, f)
}
