package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ErrCoordinatorInterrupted identifies coordinator work stopped because its Loop run left the live set.
var ErrCoordinatorInterrupted = errors.New("task: coordinator run interrupted")

// errCoordinatorLeaseLost identifies coordinator work stopped because its claim no longer holds the run.
var errCoordinatorLeaseLost = errors.New("task: coordinator lease lost")

// CoordinatorProgressRecorder advances Loop liveness while a claimed coordinator run keeps its lease.
type CoordinatorProgressRecorder interface {
	AdvanceLoopRunProgress(ctx context.Context, loopRunID string, at time.Time) error
}

type inFlightCoordinator struct {
	loopRunID string
	cancel    context.CancelCauseFunc
}

// InterruptLoopCoordinatorRuns cancels every in-flight coordinator evaluation for one Loop run and
// reports how many were interrupted. Subprocesses started under that evaluation observe the
// cancellation through their execution context, so a canceled Run cannot leave a gate command
// running against its workspace.
func (m *Service) InterruptLoopCoordinatorRuns(loopRunID string, reason string) int {
	loopRunID = strings.TrimSpace(loopRunID)
	if m == nil || loopRunID == "" {
		return 0
	}
	cause := fmt.Errorf("%w: loop run %q: %s", ErrCoordinatorInterrupted, loopRunID, coordinatorInterruptReason(reason))
	m.coordinatorMu.Lock()
	defer m.coordinatorMu.Unlock()
	interrupted := 0
	for _, inFlight := range m.inFlightCoordinators {
		if inFlight.loopRunID != loopRunID {
			continue
		}
		inFlight.cancel(cause)
		interrupted++
	}
	return interrupted
}

func coordinatorInterruptReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "coordinator evaluation interrupted"
	}
	return reason
}

// runClaimedCoordinator executes the coordinator under its claim, renewing the lease while the
// evaluation runs so the backstop cannot dispatch a second attempt for the same run.
func (m *Service) runClaimedCoordinator(
	ctx context.Context,
	run Run,
	claimToken string,
	actor ActorContext,
) (CoordinatorCompletionPlan, error) {
	execCtx, cancel := context.WithCancelCause(ctx)
	unregister := m.registerInFlightCoordinator(run, cancel)
	// A gate command can outlive the lease it started under. Recording progress before evaluation
	// starts keeps `loop runs`/`loop status` live from the first second a gate is in flight.
	m.recordCoordinatorProgress(execCtx, run.LoopRunID, m.now().UTC())
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		m.heartbeatCoordinatorLease(execCtx, cancel, run, claimToken, actor)
	}()
	plan, runErr := m.coordinatorRunner.Run(execCtx, RunID(run.ID))
	unregister()
	// An interruption or a lost claim invalidates whatever the runner computed under it.
	interruption := context.Cause(execCtx)
	cancel(nil)
	// A lost claim reaches the caller through the interruption cause the heartbeat recorded.
	<-heartbeatDone
	if interruption != nil {
		return CoordinatorCompletionPlan{}, errors.Join(interruption, runErr)
	}
	return plan, runErr
}

func (m *Service) registerInFlightCoordinator(run Run, cancel context.CancelCauseFunc) func() {
	runID := strings.TrimSpace(run.ID)
	m.coordinatorMu.Lock()
	if m.inFlightCoordinators == nil {
		m.inFlightCoordinators = make(map[string]inFlightCoordinator)
	}
	m.inFlightCoordinators[runID] = inFlightCoordinator{
		loopRunID: strings.TrimSpace(run.LoopRunID),
		cancel:    cancel,
	}
	m.coordinatorMu.Unlock()
	return func() {
		m.coordinatorMu.Lock()
		delete(m.inFlightCoordinators, runID)
		m.coordinatorMu.Unlock()
	}
}

// heartbeatCoordinatorLease renews the claim at a third of its lease duration. A lost claim
// cancels the evaluation through its cancel cause. A transient renewal failure is retried well
// before the lease expires; only a claim the store rejects stops the evaluation.
func (m *Service) heartbeatCoordinatorLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	run Run,
	claimToken string,
	actor ActorContext,
) {
	leaseDuration := coordinatorLeaseDuration(run)
	interval := coordinatorHeartbeatInterval(leaseDuration)
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		renewed, err := m.HeartbeatRunLease(ctx, LeaseHeartbeat{
			RunID:         run.ID,
			ClaimToken:    claimToken,
			LeaseDuration: leaseDuration,
			Now:           m.now().UTC(),
		}, actor)
		if ctx.Err() != nil {
			return
		}
		next := interval
		switch {
		case coordinatorLeaseLost(err):
			cancel(fmt.Errorf("%w: coordinator run %q: %w", errCoordinatorLeaseLost, run.ID, err))
			return
		case err != nil:
			slog.Warn("task: renew coordinator lease", "run_id", run.ID, "error", err)
			next = coordinatorHeartbeatRetryInterval(interval)
		case renewed != nil:
			m.recordCoordinatorProgress(ctx, renewed.LoopRunID, renewed.HeartbeatAt)
		}
		timer.Reset(next)
	}
}

func (m *Service) recordCoordinatorProgress(ctx context.Context, loopRunID string, at time.Time) {
	recorder, ok := m.store.(CoordinatorProgressRecorder)
	loopRunID = strings.TrimSpace(loopRunID)
	if !ok || loopRunID == "" {
		return
	}
	if at.IsZero() {
		at = m.now().UTC()
	}
	// Progress is advisory liveness; the renewed lease is the authoritative ownership proof.
	if err := recorder.AdvanceLoopRunProgress(ctx, loopRunID, at.UTC()); err != nil && ctx.Err() == nil {
		slog.Warn("task: advance loop progress for coordinator lease", "loop_run_id", loopRunID, "error", err)
	}
}

func coordinatorLeaseLost(err error) bool {
	return errors.Is(err, ErrInvalidClaimToken) ||
		errors.Is(err, ErrLeaseExpired) ||
		errors.Is(err, ErrInvalidStatusTransition) ||
		errors.Is(err, ErrTaskRunNotFound)
}

func coordinatorLeaseDuration(run Run) time.Duration {
	if !run.ClaimedAt.IsZero() && run.LeaseUntil.After(run.ClaimedAt) {
		return min(run.LeaseUntil.Sub(run.ClaimedAt), MaxRunLeaseDuration)
	}
	return DefaultRunLeaseDuration
}

func coordinatorHeartbeatInterval(leaseDuration time.Duration) time.Duration {
	return max(leaseDuration/3, time.Millisecond)
}

// coordinatorHeartbeatRetryInterval keeps a failed renewal inside the same lease window instead of
// waiting a full heartbeat interval, which would let the claim expire after two failures.
func coordinatorHeartbeatRetryInterval(interval time.Duration) time.Duration {
	return max(interval/3, time.Millisecond)
}
