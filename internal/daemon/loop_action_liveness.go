package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	loopActionNoProgressTimeout       = 7*time.Minute + 30*time.Second
	loopActionLivenessPollMaxInterval = time.Minute
	loopActionReasonNodeTimeout       = "node_timeout"
	loopActionReasonNoProgress        = "no_progress"
)

type loopActionSessionStatus interface {
	Status(context.Context, string) (*session.Info, error)
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
) (taskpkg.RunResult, error) {
	runCtx, cancelRun := context.WithTimeout(ctx, actionTimeout)
	usage := newLoopActionUsageState(r.now)
	runCtx = looppkg.ContextWithActionUsageReporter(runCtx, usage)
	heartbeatErrC := r.startHeartbeat(runCtx, cancelRun, claim, actor, leaseDuration, actionTimeout, usage)
	result, runErr := r.runner.ExecuteActionRun(runCtx, claim.Run, actor)
	deadlineExceeded := errors.Is(runCtx.Err(), context.DeadlineExceeded)
	cancelRun()
	heartbeatErr := <-heartbeatErrC
	if ctx.Err() != nil {
		return taskpkg.RunResult{}, ctx.Err()
	}
	if tokensUsed := usage.TokensUsed(); tokensUsed > result.TokensUsed {
		result.TokensUsed = tokensUsed
	}
	if deadlineExceeded {
		return result, errors.Join(newLoopActionLivenessError(loopActionReasonNodeTimeout), heartbeatErr, runErr)
	}
	return result, errors.Join(heartbeatErr, runErr)
}

func (r *loopActionRuntime) startHeartbeat(
	ctx context.Context,
	cancelRun context.CancelFunc,
	claim *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	leaseDuration time.Duration,
	actionTimeout time.Duration,
	usage *loopActionUsageState,
) <-chan error {
	errC := make(chan error, 1)
	go func() {
		err := r.heartbeatClaim(ctx, claim, actor, leaseDuration, actionTimeout, usage)
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
	actionTimeout time.Duration,
	usage *loopActionUsageState,
) error {
	heartbeatInterval := r.heartbeatInterval(leaseDuration)
	pollInterval := r.livenessPollInterval(actionTimeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	nextHeartbeat := time.Now().Add(heartbeatInterval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			activeTool, err := r.refreshActionProgress(ctx, usage)
			if err != nil && !errors.Is(err, context.Canceled) {
				r.logger.Warn("daemon: read loop action session activity", "error", err)
			}
			snapshot := usage.snapshot()
			if !activeTool && r.now().UTC().Sub(snapshot.progressAt) >= actionNoProgressWindow(actionTimeout) {
				return newLoopActionLivenessError(loopActionReasonNoProgress)
			}
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
	if info == nil || info.Liveness == nil || info.Liveness.Activity == nil {
		return false, nil
	}
	activity := info.Liveness.Activity
	activeTool := strings.TrimSpace(activity.CurrentTool) != "" || strings.TrimSpace(activity.ToolCallID) != ""
	if activity.LastActivityAt != nil && activity.LastActivityAt.After(snapshot.progressAt) {
		usage.mu.Lock()
		if activity.LastActivityAt.After(usage.progressAt) {
			usage.progressAt = activity.LastActivityAt.UTC()
		}
		usage.mu.Unlock()
	}
	return activeTool, nil
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
	timeout, ok, err := r.runner.ActionRunTimeout(ctx, run)
	if err != nil {
		return 0, err
	}
	if !ok {
		return r.actionRunTimeout, nil
	}
	return timeout, nil
}

func leaseDurationForActionTimeout(timeout time.Duration) time.Duration {
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

func actionNoProgressWindow(actionTimeout time.Duration) time.Duration {
	return min(loopActionNoProgressTimeout, actionTimeout/2)
}

func loopActionLivenessPollInterval(actionTimeout time.Duration) time.Duration {
	interval := actionNoProgressWindow(actionTimeout) / 3
	if interval <= 0 {
		return time.Millisecond
	}
	return min(interval, loopActionLivenessPollMaxInterval)
}

type loopActionLivenessError struct {
	reasonCode string
}

func newLoopActionLivenessError(reasonCode string) error {
	return &loopActionLivenessError{reasonCode: reasonCode}
}

func (e *loopActionLivenessError) Error() string {
	return fmt.Sprintf("loop action terminalized: %s", e.reasonCode)
}

func (e *loopActionLivenessError) loopActionReasonCode() string {
	return e.reasonCode
}

func (e *loopActionLivenessError) SafeActionFailure() looppkg.ActionFailure {
	switch e.reasonCode {
	case loopActionReasonNodeTimeout:
		return looppkg.NewActionFailure(
			e.reasonCode,
			"The action exceeded its wall-clock deadline.",
			"Review the action scope or set a larger node timeout before retrying.",
		)
	default:
		return looppkg.NewActionFailure(
			e.reasonCode,
			"The action stopped making observable progress.",
			"Inspect the bound session and retry after resolving the stalled work.",
		)
	}
}
