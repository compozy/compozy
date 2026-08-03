package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	loopActionLivenessPollMaxInterval = time.Minute
	loopActionReasonNodeTimeout       = string(looppkg.ReasonCodeActionTimeout)
	loopActionReasonNodeDeadline      = string(looppkg.FailureBudgetExhausted)
)

type loopActionSessionStatus interface {
	Status(context.Context, string) (*session.Info, error)
	Events(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
}

type loopActionSilenceWindowRunner interface {
	ActionRunSilenceWindow(context.Context, taskpkg.Run) (time.Duration, error)
}

type loopActionUsageState struct {
	tokensUsed atomic.Int64
	mu         sync.RWMutex
	sessionID  string
	progressAt time.Time
	now        func() time.Time
}

type loopActionProgressSnapshot struct {
	tokensUsed int64
	sessionID  string
	progressAt time.Time
}

func newLoopActionUsageState(now func() time.Time) *loopActionUsageState {
	return &loopActionUsageState{now: now, progressAt: now().UTC()}
}

func (s *loopActionUsageState) ReportActionTokensUsed(tokensUsed int64) {
	if s == nil || tokensUsed <= 0 {
		return
	}
	for {
		current := s.tokensUsed.Load()
		if tokensUsed <= current {
			return
		}
		if s.tokensUsed.CompareAndSwap(current, tokensUsed) {
			s.recordProgress()
			return
		}
	}
}

func (s *loopActionUsageState) ReportActionSessionBound(sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.sessionID = strings.TrimSpace(sessionID)
	s.progressAt = s.now().UTC()
	s.mu.Unlock()
}

func (s *loopActionUsageState) recordProgress() {
	s.mu.Lock()
	s.progressAt = s.now().UTC()
	s.mu.Unlock()
}

func (s *loopActionUsageState) snapshot() loopActionProgressSnapshot {
	if s == nil {
		return loopActionProgressSnapshot{}
	}
	s.mu.RLock()
	snapshot := loopActionProgressSnapshot{
		tokensUsed: s.tokensUsed.Load(),
		sessionID:  s.sessionID,
		progressAt: s.progressAt,
	}
	s.mu.RUnlock()
	return snapshot
}

func (s *loopActionUsageState) TokensUsed() int64 {
	if s == nil {
		return 0
	}
	return s.tokensUsed.Load()
}

func (r *loopActionRuntime) executeClaimedRun(
	ctx context.Context,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	leaseDuration time.Duration,
	actionTimeout time.Duration,
	timeoutReason string,
	workspaceID looppkg.WorkspaceID,
	silenceWindow time.Duration,
	deathStreakLimit int,
) (taskpkg.RunResult, bool, error) {
	runCtx, cancelRun := loopActionExecutionContext(ctx, actionTimeout)
	usage := newLoopActionUsageState(r.now)
	runCtx = looppkg.ContextWithActionUsageReporter(runCtx, usage)
	heartbeatErrC := r.startHeartbeat(
		runCtx,
		cancelRun,
		claim,
		actor,
		leaseDuration,
		usage,
		workspaceID,
		silenceWindow,
	)
	result, runErr := r.runner.ExecuteActionRun(runCtx, claim.Run, actor)
	deadlineExceeded := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	cancelRun()
	heartbeatErr := <-heartbeatErrC
	if ctx.Err() != nil {
		return taskpkg.RunResult{}, false, ctx.Err()
	}
	if tokensUsed := usage.TokensUsed(); tokensUsed > result.TokensUsed {
		result.TokensUsed = tokensUsed
	}
	if deadlineExceeded {
		return result, false, errors.Join(newLoopActionTimeoutError(timeoutReason), heartbeatErr, runErr)
	}
	if runErr != nil {
		handled, resumeErr := r.resumeConfirmedDeadAction(
			ctx, claim.Run, actor, workspaceID, usage.snapshot().sessionID, deathStreakLimit,
		)
		if handled {
			return result, true, errors.Join(heartbeatErr, resumeErr)
		}
		runErr = errors.Join(runErr, resumeErr)
	}
	return result, false, errors.Join(heartbeatErr, runErr)
}

func loopActionExecutionContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func (r *loopActionRuntime) startHeartbeat(
	ctx context.Context,
	cancelRun context.CancelFunc,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	leaseDuration time.Duration,
	usage *loopActionUsageState,
	workspaceID looppkg.WorkspaceID,
	silenceWindow time.Duration,
) <-chan error {
	errC := make(chan error, 1)
	go func() {
		err := r.heartbeatClaim(
			ctx,
			claim,
			actor,
			leaseDuration,
			usage,
			workspaceID,
			silenceWindow,
		)
		if err != nil {
			cancelRun()
		}
		errC <- err
	}()
	return errC
}

func (r *loopActionRuntime) heartbeatClaim(
	ctx context.Context,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	leaseDuration time.Duration,
	usage *loopActionUsageState,
	workspaceID looppkg.WorkspaceID,
	silenceWindow time.Duration,
) error {
	heartbeatInterval := r.heartbeatInterval(leaseDuration)
	pollInterval := r.livenessPollInterval(leaseDuration)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	nextHeartbeat := time.Now().Add(heartbeatInterval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			evidence, err := r.refreshActionProgress(ctx, usage)
			if err != nil && !errors.Is(err, context.Canceled) {
				r.logger.Warn("daemon: read loop action session activity", "error", err)
			} else if err == nil {
				if recordErr := r.recordActionLiveness(
					ctx, claim.Run, workspaceID, evidence, silenceWindow,
				); recordErr != nil && !errors.Is(recordErr, context.Canceled) {
					r.logger.Warn("daemon: persist loop action liveness", "error", recordErr)
				}
			}
			snapshot := usage.snapshot()
			if time.Now().Before(nextHeartbeat) {
				continue
			}
			if err := r.extendActionLease(ctx, claim, actor, leaseDuration, snapshot.tokensUsed); err != nil {
				return err
			}
			nextHeartbeat = time.Now().Add(heartbeatInterval)
		}
	}
}

func (r *loopActionRuntime) refreshActionProgress(
	ctx context.Context,
	usage *loopActionUsageState,
) (bool, error) {
	snapshot := usage.snapshot()
	if r.sessions == nil || snapshot.sessionID == "" {
		return false, nil
	}
	info, err := r.sessions.Status(ctx, snapshot.sessionID)
	if err != nil {
		return false, err
	}
	if info == nil {
		return false, nil
	}
	if info.Liveness == nil || info.Liveness.Activity == nil {
		return true, nil
	}
	activity := info.Liveness.Activity
	if activity.LastActivityAt != nil && activity.LastActivityAt.After(snapshot.progressAt) {
		usage.mu.Lock()
		if activity.LastActivityAt.After(usage.progressAt) {
			usage.progressAt = activity.LastActivityAt.UTC()
		}
		usage.mu.Unlock()
	}
	return true, nil
}

func (r *loopActionRuntime) recordActionLiveness(
	ctx context.Context,
	run taskpkg.Run,
	workspaceID looppkg.WorkspaceID,
	evidence bool,
	silenceWindow time.Duration,
) error {
	recorder, ok := r.store.(looppkg.NodeLivenessRecorder)
	if !ok {
		return nil
	}
	return recorder.RecordNodeLiveness(ctx, looppkg.NodeLivenessObservation{
		WorkspaceID:  workspaceID,
		LoopRunID:    looppkg.RunID(strings.TrimSpace(run.LoopRunID)),
		TaskRunID:    strings.TrimSpace(run.ID),
		ObservedAt:   r.now().UTC(),
		Evidence:     evidence,
		SilenceAfter: silenceWindow,
	})
}

func (r *loopActionRuntime) extendActionLease(
	ctx context.Context,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	leaseDuration time.Duration,
	tokensUsed int64,
) error {
	_, err := r.manager.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         claim.Run.ID,
		ClaimToken:    claim.ClaimToken,
		LeaseDuration: leaseDuration,
		Now:           r.now().UTC(),
		TokensUsed:    tokensUsed,
	}, actor)
	if err != nil && ctx.Err() != nil {
		return nil
	}
	return err
}

func (r *loopActionRuntime) actionTimeoutForRun(
	ctx context.Context,
	run taskpkg.Run,
) (time.Duration, error) {
	timeout, authored, err := r.actionTimeoutSpecForRun(ctx, run)
	if err != nil || authored {
		return timeout, err
	}
	return 0, nil
}

func (r *loopActionRuntime) actionTimeoutSpecForRun(
	ctx context.Context,
	run taskpkg.Run,
) (time.Duration, bool, error) {
	timeout, authored, err := r.runner.ActionRunTimeout(ctx, run)
	if err != nil {
		return 0, false, err
	}
	return timeout, authored, nil
}

func (r *loopActionRuntime) actionSilenceWindowForRun(
	ctx context.Context,
	run taskpkg.Run,
) (time.Duration, error) {
	provider, ok := r.runner.(loopActionSilenceWindowRunner)
	if !ok {
		defaults := looppkg.DefaultLifecycleConfig()
		return *defaults.LivenessSilenceWindow, nil
	}
	return provider.ActionRunSilenceWindow(ctx, run)
}

type loopActionDeathStreakRunner interface {
	ActionRunDeathStreakLimit(context.Context, taskpkg.Run) (int, error)
}

func (r *loopActionRuntime) actionDeathStreakLimitForRun(
	ctx context.Context,
	run taskpkg.Run,
) (int, error) {
	provider, ok := r.runner.(loopActionDeathStreakRunner)
	if !ok {
		defaults := looppkg.DefaultLifecycleConfig()
		return *defaults.ResumeDeathStreakLimit, nil
	}
	return provider.ActionRunDeathStreakLimit(ctx, run)
}

type loopActionTimeoutReasonRunner interface {
	ActionRunTimeoutReason(context.Context, taskpkg.Run) (string, error)
}

func (r *loopActionRuntime) actionTimeoutReasonForRun(
	ctx context.Context,
	run taskpkg.Run,
) (string, error) {
	reasonRunner, ok := r.runner.(loopActionTimeoutReasonRunner)
	if !ok {
		return loopActionReasonNodeTimeout, nil
	}
	reason, err := reasonRunner.ActionRunTimeoutReason(ctx, run)
	if err != nil {
		return "", err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return loopActionReasonNodeTimeout, nil
	}
	return reason, nil
}

func leaseDurationForActionTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return taskpkg.DefaultRunLeaseDuration
	}
	leaseDuration := max(taskpkg.DefaultRunLeaseDuration, timeout)
	return min(leaseDuration, taskpkg.MaxRunLeaseDuration)
}

func loopActionHeartbeatInterval(leaseDuration time.Duration) time.Duration {
	interval := leaseDuration / 3
	if interval <= 0 {
		return time.Second
	}
	return min(interval, loopActionHeartbeatMaxInterval)
}

func loopActionLivenessPollInterval(leaseDuration time.Duration) time.Duration {
	interval := loopActionHeartbeatInterval(leaseDuration) / 3
	if interval <= 0 {
		return time.Millisecond
	}
	return min(interval, loopActionLivenessPollMaxInterval)
}

type loopActionTimeoutError struct {
	reasonCode string
}

func newLoopActionTimeoutError(reasonCode string) error {
	return &loopActionTimeoutError{reasonCode: reasonCode}
}

func (e *loopActionTimeoutError) Error() string {
	return fmt.Sprintf("loop action terminalized: %s", e.reasonCode)
}

func (e *loopActionTimeoutError) loopActionReasonCode() string {
	return e.reasonCode
}

func (e *loopActionTimeoutError) SafeActionFailure() looppkg.ActionFailure {
	switch e.reasonCode {
	case loopActionReasonNodeTimeout:
		return looppkg.NewActionFailure(
			e.reasonCode,
			"The action exceeded its wall-clock deadline.",
			"Review the action scope or set a larger node timeout before retrying.",
		)
	case loopActionReasonNodeDeadline:
		return looppkg.NewActionFailure(
			e.reasonCode,
			"The action exhausted its authored total deadline.",
			"Review the total deadline and prior attempts before starting a new generation.",
		)
	default:
		return looppkg.NewActionFailure(
			e.reasonCode,
			"The action exceeded its authored time limit.",
			"Review the authored timeout or deadline before retrying.",
		)
	}
}
