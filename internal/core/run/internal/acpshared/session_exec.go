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
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
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
	return ExecuteSessionAndResolve(attemptCtx, cfg, timeout, execution, j, index, emitHuman)
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
	cfg *config,
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

	controller := newSessionTurnController(ctx, cfg, timeout, execution, j, index, emitHuman)
	unregister := func() {}
	if cfg != nil && cfg.JobControls != nil {
		unregister = cfg.JobControls.Register(cfg.RunArtifacts.RunID, index, safeJobID(j), controller)
	}
	defer unregister()
	return controller.run()
}

func executeSingleSessionTurn(
	ctx context.Context,
	timeout time.Duration,
	execution *SessionExecution,
	j *job,
	index int,
	emitHuman bool,
	pauseRequested func() bool,
) (JobAttemptResult, bool) {
	if execution == nil || execution.Session == nil {
		return handleNilExecution(j, index, emitHuman), false
	}

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
			return BuildFailureResult(streamErr, -1, j, index, emitHuman), false
		}

		sessionErr := execution.Session.Err()
		if agent.IsPromptCancelled(sessionErr) && pauseRequested != nil && pauseRequested() {
			return JobAttemptResult{}, true
		}
		if err := execution.Handler.HandleCompletion(sessionErr); err != nil {
			execution.Logger.Warn("failed to finalize ACP session handler", "error", err)
		}
		if sessionErr != nil {
			appendLinesToBuffer(j.ErrBuffer, []string{"ACP session error: " + sessionErr.Error()})
		}
		return handleSessionCompletion(ctx, sessionErr, timeout, j, index, emitHuman), false
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
			), false
		}
		return HandleSessionCancellation(cancelErr, j, index, emitHuman), false
	}
}

type sessionTurnState string

const (
	sessionTurnStateRunning sessionTurnState = "running"
	sessionTurnStatePausing sessionTurnState = "pausing"
	sessionTurnStatePaused  sessionTurnState = "paused"
	sessionTurnStateClosed  sessionTurnState = "closed"
)

type sessionResumeRequest struct {
	message   string
	messageID string
	reply     chan sessionResumeResult
}

type sessionResumeResult struct {
	response model.JobControlResponse
	err      error
}

type sessionTurnController struct {
	ctx       context.Context
	cfg       *config
	timeout   time.Duration
	execution *SessionExecution
	job       *job
	index     int
	emitHuman bool

	mu             sync.Mutex
	state          sessionTurnState
	resuming       bool
	pauseRequested bool
	resumeCh       chan sessionResumeRequest
}

var _ model.JobController = (*sessionTurnController)(nil)

func newSessionTurnController(
	ctx context.Context,
	cfg *config,
	timeout time.Duration,
	execution *SessionExecution,
	j *job,
	index int,
	emitHuman bool,
) *sessionTurnController {
	if ctx == nil {
		ctx = context.Background()
	}
	return &sessionTurnController{
		ctx:       ctx,
		cfg:       cfg,
		timeout:   timeout,
		execution: execution,
		job:       j,
		index:     index,
		emitHuman: emitHuman,
		state:     sessionTurnStateRunning,
		resumeCh:  make(chan sessionResumeRequest),
	}
}

func (c *sessionTurnController) Pause(
	ctx context.Context,
	_ model.JobControlRequest,
) (model.JobControlResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	sessionID := c.sessionIDLocked()
	switch c.state {
	case sessionTurnStatePaused:
		if c.resuming {
			c.mu.Unlock()
			return model.JobControlResponse{}, fmt.Errorf("%w: job is resuming", model.ErrJobControlConflict)
		}
		resp := c.responseLocked(model.JobControlStatusPaused, "")
		c.mu.Unlock()
		return resp, nil
	case sessionTurnStatePausing:
		resp := c.responseLocked(model.JobControlStatusPausing, "")
		c.mu.Unlock()
		return resp, nil
	case sessionTurnStateRunning:
		if sessionID == "" {
			c.mu.Unlock()
			return model.JobControlResponse{}, fmt.Errorf("%w: session is not ready", model.ErrJobControlConflict)
		}
		c.state = sessionTurnStatePausing
		c.pauseRequested = true
		resp := c.responseLocked(model.JobControlStatusPausing, "")
		c.mu.Unlock()
		if err := c.execution.Client.CancelSession(ctx, sessionID); err != nil {
			c.revertPausing()
			return model.JobControlResponse{}, err
		}
		if err := c.emitJobPausing(sessionID); err != nil {
			c.execution.Logger.Warn("failed to emit job pausing event", "error", err)
		}
		return resp, nil
	default:
		state := c.state
		c.mu.Unlock()
		return model.JobControlResponse{}, fmt.Errorf("%w: job is %s", model.ErrJobControlConflict, state)
	}
}

func (c *sessionTurnController) SendMessage(
	ctx context.Context,
	req model.JobControlRequest,
) (model.JobControlResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return model.JobControlResponse{}, model.ErrJobControlMessageRequired
	}
	messageID, err := agent.NewPromptMessageID()
	if err != nil {
		return model.JobControlResponse{}, err
	}
	reply := make(chan sessionResumeResult, 1)
	resume := sessionResumeRequest{
		message:   message,
		messageID: messageID,
		reply:     reply,
	}
	c.mu.Lock()
	if c.state != sessionTurnStatePaused {
		state := c.state
		c.mu.Unlock()
		return model.JobControlResponse{}, fmt.Errorf("%w: job is %s", model.ErrJobControlConflict, state)
	}
	if c.resuming {
		c.mu.Unlock()
		return model.JobControlResponse{}, fmt.Errorf("%w: job is resuming", model.ErrJobControlConflict)
	}
	c.resuming = true
	resumeCh := c.resumeCh
	c.mu.Unlock()

	select {
	case resumeCh <- resume:
	case <-ctx.Done():
		c.restorePaused()
		return model.JobControlResponse{}, ctx.Err()
	}
	select {
	case result := <-reply:
		return result.response, result.err
	case <-ctx.Done():
		return model.JobControlResponse{}, ctx.Err()
	}
}

func (c *sessionTurnController) run() JobAttemptResult {
	defer c.markClosed()
	for {
		result, paused := executeSingleSessionTurn(
			c.ctx,
			c.timeout,
			c.execution,
			c.job,
			c.index,
			c.emitHuman,
			c.pauseRequestedLocked,
		)
		if !paused {
			return result
		}
		if err := c.markPaused(); err != nil {
			return BuildFailureResult(err, -1, c.job, c.index, c.emitHuman)
		}
		resume, ok := c.waitForResume()
		if !ok {
			cancelErr := context.Cause(c.ctx)
			if cancelErr == nil {
				cancelErr = c.ctx.Err()
			}
			return HandleSessionCancellation(cancelErr, c.job, c.index, c.emitHuman)
		}
		if err := c.startResume(resume); err != nil {
			resume.reply <- sessionResumeResult{err: err}
			c.restorePaused()
			continue
		}
	}
}

func (c *sessionTurnController) waitForResume() (sessionResumeRequest, bool) {
	select {
	case resume := <-c.resumeCh:
		return resume, true
	case <-c.ctx.Done():
		return sessionResumeRequest{}, false
	}
}

func (c *sessionTurnController) startResume(resume sessionResumeRequest) error {
	sessionID := c.sessionID()
	if sessionID == "" {
		return fmt.Errorf("%w: session is not ready", model.ErrJobControlConflict)
	}
	session, err := c.execution.Client.PromptSession(c.ctx, agent.PromptSessionRequest{
		SessionID: sessionID,
		Prompt:    []byte(resume.message),
		MessageID: resume.messageID,
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.execution.Session = session
	c.mu.Unlock()
	if err := c.emitUserMessage(resume.messageID, resume.message); err != nil {
		c.execution.Logger.Warn("failed to emit local user message update", "error", err)
	}
	if c.execution.Activity != nil {
		c.execution.Activity.EndActivity()
	}
	c.mu.Lock()
	c.state = sessionTurnStateRunning
	c.resuming = false
	c.pauseRequested = false
	resp := c.responseLocked(model.JobControlStatusResumed, resume.messageID)
	c.mu.Unlock()
	if err := c.emitJobResumed(sessionID, resume.messageID); err != nil {
		c.execution.Logger.Warn("failed to emit job resumed event", "error", err)
	}
	resume.reply <- sessionResumeResult{response: resp}
	return nil
}

func (c *sessionTurnController) emitUserMessage(messageID string, message string) error {
	block, err := model.NewContentBlock(model.TextBlock{Text: message})
	if err != nil {
		return err
	}
	return c.execution.Handler.HandleUpdate(model.SessionUpdate{
		Kind:      model.UpdateKindUserMessageChunk,
		MessageID: messageID,
		Blocks:    []model.ContentBlock{block},
		Status:    model.StatusRunning,
	})
}

func (c *sessionTurnController) markPaused() error {
	sessionID := c.sessionID()
	if sessionID == "" {
		return fmt.Errorf("%w: session is not ready", model.ErrJobControlConflict)
	}
	if c.execution.Activity != nil {
		c.execution.Activity.BeginActivity()
	}
	c.mu.Lock()
	c.state = sessionTurnStatePaused
	c.pauseRequested = false
	c.mu.Unlock()
	return c.emitJobPaused(sessionID)
}

func (c *sessionTurnController) restorePaused() {
	c.mu.Lock()
	c.state = sessionTurnStatePaused
	c.resuming = false
	c.pauseRequested = false
	c.mu.Unlock()
}

func (c *sessionTurnController) revertPausing() {
	c.mu.Lock()
	if c.state == sessionTurnStatePausing {
		c.state = sessionTurnStateRunning
	}
	c.pauseRequested = false
	c.mu.Unlock()
}

func (c *sessionTurnController) markClosed() {
	c.mu.Lock()
	c.state = sessionTurnStateClosed
	c.resuming = false
	c.pauseRequested = false
	c.mu.Unlock()
}

func (c *sessionTurnController) pauseRequestedLocked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pauseRequested
}

func (c *sessionTurnController) sessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionIDLocked()
}

func (c *sessionTurnController) sessionIDLocked() string {
	if c == nil || c.execution == nil || c.execution.Session == nil {
		return ""
	}
	return strings.TrimSpace(c.execution.Session.ID())
}

func (c *sessionTurnController) responseLocked(
	status model.JobControlStatus,
	messageID string,
) model.JobControlResponse {
	runID := ""
	if c.cfg != nil {
		runID = c.cfg.RunArtifacts.RunID
	}
	return model.JobControlResponse{
		RunID:     runID,
		JobID:     firstNonEmpty(safeJobID(c.job), fmt.Sprintf("job-%03d", c.index)),
		Index:     c.index,
		Status:    status,
		SessionID: c.sessionIDLocked(),
		MessageID: strings.TrimSpace(messageID),
	}
}

func (c *sessionTurnController) emitJobPausing(sessionID string) error {
	return c.execution.Handler.submitRuntimeEvent(
		events.EventKindJobPausing,
		kinds.JobPausingPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: c.index},
			SessionID:      sessionID,
		},
		"job pausing",
	)
}

func (c *sessionTurnController) emitJobPaused(sessionID string) error {
	return c.execution.Handler.submitRuntimeEvent(
		events.EventKindJobPaused,
		kinds.JobPausedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: c.index},
			SessionID:      sessionID,
		},
		"job paused",
	)
}

func (c *sessionTurnController) emitJobResumed(sessionID string, messageID string) error {
	return c.execution.Handler.submitRuntimeEvent(
		events.EventKindJobResumed,
		kinds.JobResumedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: c.index},
			SessionID:      sessionID,
			MessageID:      messageID,
		},
		"job resumed",
	)
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
	if agent.IsAuthenticationRequired(err) {
		return false
	}
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
