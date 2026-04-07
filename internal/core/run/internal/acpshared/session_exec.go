package acpshared

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

func ExecuteJobWithTimeout(
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
) JobAttemptResult {
	emitHuman := cfg.HumanOutputEnabled() && !useUI
	attemptCtx := ctx
	cancel := func(error) {}
	stopActivityWatchdog := func() {}
	var activity *activityMonitor
	if timeout > 0 {
		activity = newActivityMonitor()
		attemptCtx, cancel = context.WithCancelCause(ctx)
		stopActivityWatchdog = StartACPActivityWatchdog(attemptCtx, activity, timeout, cancel)
	}
	defer func() {
		stopActivityWatchdog()
		cancel(nil)
	}()

	execution, err := SetupSessionExecution(SessionSetupRequest{
		Context:           attemptCtx,
		Config:            cfg,
		Job:               j,
		CWD:               cwd,
		UseUI:             useUI,
		StreamHumanOutput: cfg.HumanOutputEnabled(),
		Index:             index,
		RunJournal:        runJournal,
		AggregateUsage:    aggregateUsage,
		AggregateMu:       aggregateMu,
		Activity:          activity,
		Logger:            runtimeLoggerFor(cfg, useUI),
		TrackClient:       trackClient,
	})
	if err != nil {
		if timeout > 0 && IsActivityTimeout(err) {
			return HandleSessionTimeout(ResolveTimeoutError(timeout, err), j, index, emitHuman, timeout)
		}
		fail := RecordFailureWithContext(nil, j, nil, err, -1)
		return jobAttemptResult{
			Status:    attemptStatusSetupFailed,
			ExitCode:  -1,
			Failure:   &fail,
			Retryable: RetryableSetupFailure(err),
		}
	}
	return ExecuteSessionAndResolve(attemptCtx, timeout, execution, j, index, emitHuman)
}

type activityTimeoutError struct {
	timeout time.Duration
}

type ActivityTimeoutError = activityTimeoutError

func NewActivityTimeoutError(timeout time.Duration) error {
	return &activityTimeoutError{timeout: timeout}
}

func (e *activityTimeoutError) Error() string {
	return fmt.Sprintf("activity timeout: no output received for %v", e.timeout)
}

func StartACPActivityWatchdog(
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
				if monitor.TimeSinceLastActivity() > timeout {
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

func CreateLogFile(path string) (*os.File, error) {
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
	codeFileLabel := j.CodeFileLabel()
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: -1,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      fmt.Errorf("failed to set up ACP session execution"),
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Failed to set up job %d (%s): %v\n", index+1, codeFileLabel, failure.Err)
	}
	return jobAttemptResult{Status: attemptStatusSetupFailed, ExitCode: -1, Failure: &failure}
}

func ExecuteSessionAndResolve(
	ctx context.Context,
	timeout time.Duration,
	execution *SessionExecution,
	j *job,
	index int,
	emitHuman bool,
) JobAttemptResult {
	if execution == nil || execution.Session == nil {
		return handleNilExecution(j, index, emitHuman)
	}
	defer execution.Close()

	streamErrCh := make(chan error, 1)
	go func() {
		streamErrCh <- StreamSessionUpdates(execution.Session, execution.Handler)
	}()

	select {
	case <-execution.Session.Done():
		streamErr := <-streamErrCh
		if streamErr != nil {
			if err := execution.Handler.HandleCompletion(streamErr); err != nil {
				execution.Logger.Warn("failed to finalize ACP session handler after stream error", "error", err)
			}
			appendLinesToBuffer(j.ErrBuffer, []string{"ACP session error: " + streamErr.Error()})
			return BuildFailureResult(streamErr, -1, j, index, emitHuman)
		}

		sessionErr := execution.Session.Err()
		if err := execution.Handler.HandleCompletion(sessionErr); err != nil {
			execution.Logger.Warn("failed to finalize ACP session handler", "error", err)
		}
		if sessionErr != nil {
			appendLinesToBuffer(j.ErrBuffer, []string{"ACP session error: " + sessionErr.Error()})
		}
		return handleSessionCompletion(ctx, sessionErr, timeout, j, index, emitHuman)
	case <-ctx.Done():
		cancelErr := context.Cause(ctx)
		if cancelErr == nil {
			cancelErr = ctx.Err()
		}
		if err := execution.Handler.HandleCompletion(cancelErr); err != nil {
			execution.Logger.Warn("failed to finalize ACP session handler after context cancellation", "error", err)
		}
		appendLinesToBuffer(j.ErrBuffer, []string{"ACP session error: " + cancelErr.Error()})
		if isSessionTimeout(ctx, cancelErr) {
			return HandleSessionTimeout(
				ResolveTimeoutError(timeout, cancelErr, context.Cause(ctx), ctx.Err()),
				j,
				index,
				emitHuman,
				timeout,
			)
		}
		return HandleSessionCancellation(cancelErr, j, index, emitHuman)
	}
}

func StreamSessionUpdates(session agent.Session, handler *SessionUpdateHandler) error {
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
		return jobAttemptResult{Status: attemptStatusSuccess, ExitCode: 0}
	}

	if isSessionTimeout(ctx, sessionErr) {
		return HandleSessionTimeout(
			ResolveTimeoutError(timeout, sessionErr, context.Cause(ctx), ctx.Err()),
			j,
			index,
			emitHuman,
			timeout,
		)
	}
	if errors.Is(sessionErr, context.Canceled) {
		return HandleSessionCancellation(sessionErr, j, index, emitHuman)
	}

	exitCode := SessionErrorCode(sessionErr)
	return BuildFailureResult(sessionErr, exitCode, j, index, emitHuman)
}

func isSessionTimeout(ctx context.Context, err error) bool {
	return IsActivityTimeout(err) ||
		IsActivityTimeout(context.Cause(ctx)) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func IsActivityTimeout(err error) bool {
	var timeoutErr *activityTimeoutError
	return errors.As(err, &timeoutErr)
}

func ResolveTimeoutError(timeout time.Duration, errs ...error) error {
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

func HandleSessionCancellation(
	cancelErr error,
	j *job,
	index int,
	emitHuman bool,
) JobAttemptResult {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nCanceling job %d (%s) due to shutdown signal\n",
			index+1,
			j.CodeFileLabel(),
		)
	}
	codeFileLabel := j.CodeFileLabel()
	if cancelErr == nil {
		cancelErr = fmt.Errorf("job canceled by shutdown")
	}
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: exitCodeCanceled,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      cancelErr,
	}
	return jobAttemptResult{Status: attemptStatusCanceled, ExitCode: exitCodeCanceled, Failure: &failure}
}

func HandleSessionTimeout(
	timeoutErr error,
	j *job,
	index int,
	emitHuman bool,
	timeout time.Duration,
) JobAttemptResult {
	logTimeoutMessage(index, j, timeout, emitHuman)
	codeFileLabel := j.CodeFileLabel()
	if timeoutErr == nil {
		timeoutErr = fmt.Errorf("ACP session timeout after %v", timeout)
	}
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: exitCodeTimeout,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      timeoutErr,
	}
	return jobAttemptResult{
		Status:    attemptStatusTimeout,
		ExitCode:  exitCodeTimeout,
		Failure:   &failure,
		Retryable: true,
	}
}

func logTimeoutMessage(index int, j *job, timeout time.Duration, emitHuman bool) {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nJob %d (%s) timed out after %v of inactivity\n",
			index+1,
			j.CodeFileLabel(),
			timeout,
		)
	}
}

func BuildFailureResult(err error, exitCode int, j *job, index int, emitHuman bool) JobAttemptResult {
	codeFileLabel := j.CodeFileLabel()
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: exitCode,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      err,
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Job %d (%s) failed with code %d: %v\n", index+1, codeFileLabel, exitCode, err)
	}
	return jobAttemptResult{
		Status:    attemptStatusFailure,
		ExitCode:  exitCode,
		Failure:   &failure,
		Retryable: true,
	}
}

func RetryableSetupFailure(err error) bool {
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

func SessionErrorCode(err error) int {
	var sessionErr *agent.SessionError
	if errors.As(err, &sessionErr) {
		return sessionErr.Code
	}
	return -1
}

func RecordFailureWithContext(
	failuresMu *sync.Mutex,
	j *job,
	failures *[]FailInfo,
	err error,
	exitCode int,
) FailInfo {
	codeFileLabel := j.CodeFileLabel()
	failure := failInfo{
		CodeFile: codeFileLabel,
		ExitCode: exitCode,
		OutLog:   j.OutLog,
		ErrLog:   j.ErrLog,
		Err:      err,
	}
	RecordFailure(failuresMu, failures, failure)
	return failure
}

func RecordFailure(mu *sync.Mutex, list *[]FailInfo, f FailInfo) {
	if list == nil {
		return
	}
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	*list = append(*list, f)
}
