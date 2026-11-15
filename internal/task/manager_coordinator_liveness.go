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

// LoopRunLivenessReader reports whether a Loop run still accepts coordinator work. The task manager
// consumes it as an optional store capability, so a manager without a Loop-owning store keeps its
// in-process behavior.
type LoopRunLivenessReader interface {
	CoordinatedLoopRunLive(ctx context.Context, loopRunID string) (bool, error)
}

type inFlightCoordinator struct {
	loopRunID string
	cancel    context.CancelCauseFunc
}

// coordinatorEvaluationKey identifies one in-flight coordinator evaluation. The key must be unique
// per evaluation, not per run: an expired lease requeues the same run ID, so a replacement can start
// while the superseded attempt is still returning.
type coordinatorEvaluationKey struct {
	runID string
	seq   uint64
}

// InterruptLoopCoordinatorRuns cancels every in-flight coordinator evaluation for one Loop run and
// reports how many were interrupted, including a replacement that runs alongside a superseded
// attempt whose lease already expired. Subprocesses started under that evaluation observe the
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
	// Cancellation can commit between the claim and the registration above, when the sweep had no
	// entry to interrupt. A registered evaluation therefore re-reads the Loop run before doing any
	// work, so it cannot start a gate command for a canceled Run.
	if !m.loopRunAcceptsCoordinator(execCtx, run.LoopRunID) {
		unregister()
		cancel(nil)
		return CoordinatorCompletionPlan{}, fmt.Errorf(
			"%w: loop run %q is no longer running",
			ErrCoordinatorInterrupted,
			strings.TrimSpace(run.LoopRunID),
		)
	}
	// A gate command can outlive the lease it started under. Recording progress before evaluation
	// starts keeps `loop runs`/`loop status` live from the first second a gate is in flight.
	m.recordCoordinatorProgress(execCtx, run.LoopRunID, m.now().UTC())
	heartbeatDone := make(chan error, 1)
	go func() {
		heartbeatDone <- m.heartbeatCoordinatorLease(execCtx, cancel, run, claimToken, actor)
	}()
	plan, runErr := m.coordinatorRunner.Run(execCtx, RunID(run.ID))
	unregister()
	// An interruption or a lost claim invalidates whatever the runner computed under it.
	interruption := context.Cause(execCtx)
	cancel(nil)
	return coordinatorEvaluationOutcome(plan, runErr, interruption, <-heartbeatDone)
}

// coordinatorEvaluationOutcome discards a plan computed under an interruption or a lost claim. The
// heartbeat's own lease-loss error is joined in because the runner can finish first: the wrapper's
// cancel(nil) then sets the cause, so the heartbeat's later cancel cannot record the loss.
func coordinatorEvaluationOutcome(
	plan CoordinatorCompletionPlan,
	runErr error,
	interruption error,
	heartbeatErr error,
) (CoordinatorCompletionPlan, error) {
	switch {
	case interruption == nil:
		interruption = heartbeatErr
	case heartbeatErr == nil || errors.Is(interruption, heartbeatErr):
		// The heartbeat's own cancel already recorded this loss; a second copy only repeats it.
	default:
		interruption = errors.Join(interruption, heartbeatErr)
	}
	if interruption != nil {
		return CoordinatorCompletionPlan{}, errors.Join(interruption, runErr)
	}
	return plan, runErr
}

// loopRunAcceptsCoordinator re-reads Loop-run liveness through the optional store capability. A read
// failure leaves the evaluation running, because the renewed claim stays the ownership proof.
func (m *Service) loopRunAcceptsCoordinator(ctx context.Context, loopRunID string) bool {
	reader, ok := m.store.(LoopRunLivenessReader)
	loopRunID = strings.TrimSpace(loopRunID)
	if !ok || loopRunID == "" {
		return true
	}
	live, err := reader.CoordinatedLoopRunLive(ctx, loopRunID)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("task: re-read loop run liveness for coordinator", "loop_run_id", loopRunID, "error", err)
		}
		return true
	}
	return live
}

func (m *Service) registerInFlightCoordinator(run Run, cancel context.CancelCauseFunc) func() {
	runID := strings.TrimSpace(run.ID)
	m.coordinatorMu.Lock()
	if m.inFlightCoordinators == nil {
		m.inFlightCoordinators = make(map[coordinatorEvaluationKey]inFlightCoordinator)
	}
	m.coordinatorSeq++
	key := coordinatorEvaluationKey{runID: runID, seq: m.coordinatorSeq}
	m.inFlightCoordinators[key] = inFlightCoordinator{
		loopRunID: strings.TrimSpace(run.LoopRunID),
		cancel:    cancel,
	}
	m.coordinatorMu.Unlock()
	// Unregistering deletes only this evaluation's key. A replacement registered for the same run
	// ID while this evaluation finishes therefore stays reachable for interruption.
	return func() {
		m.coordinatorMu.Lock()
		delete(m.inFlightCoordinators, key)
		m.coordinatorMu.Unlock()
	}
}

// heartbeatCoordinatorLease renews the claim at a third of its lease duration. A lost claim
// cancels the evaluation through its cancel cause and is also returned, so the caller keeps the
// loss even when the evaluation finishes first. A transient renewal failure is retried well before
// the lease expires; only a claim the store rejects stops the evaluation.
func (m *Service) heartbeatCoordinatorLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	run Run,
	claimToken string,
	actor ActorContext,
) error {
	leaseDuration := coordinatorLeaseDuration(run)
	interval := coordinatorHeartbeatInterval(leaseDuration)
	timer := time.NewTimer(coordinatorFirstHeartbeatDelay(interval, run.LeaseUntil, m.now().UTC()))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		renewed, err := m.HeartbeatRunLease(ctx, LeaseHeartbeat{
			RunID:         run.ID,
			ClaimToken:    claimToken,
			LeaseDuration: leaseDuration,
			Now:           m.now().UTC(),
		}, actor)
		if ctx.Err() != nil {
			return nil
		}
		next, stop := coordinatorHeartbeatStep(interval, err)
		if stop {
			leaseErr := fmt.Errorf("%w: coordinator run %q: %w", errCoordinatorLeaseLost, run.ID, err)
			cancel(leaseErr)
			return leaseErr
		}
		switch {
		case err != nil:
			slog.Warn("task: renew coordinator lease", "run_id", run.ID, "error", err)
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

// coordinatorFirstHeartbeatDelay keeps the first renewal inside the claimed lease. The transitions
// and the initial progress write can consume part of the lease before the heartbeat starts, so the
// wait follows what is left of it instead of a full interval; later renewals keep that cadence.
func coordinatorFirstHeartbeatDelay(interval time.Duration, leaseUntil time.Time, now time.Time) time.Duration {
	const minimumDelay = time.Millisecond
	if interval <= 0 {
		return minimumDelay
	}
	if leaseUntil.IsZero() || now.IsZero() {
		return interval
	}
	remaining := leaseUntil.Sub(now.UTC())
	if remaining < minimumDelay {
		return minimumDelay
	}
	return min(remaining, interval)
}

// coordinatorHeartbeatRetryInterval keeps a failed renewal inside the same lease window instead of
// waiting a full heartbeat interval, which would let the claim expire after two failures.
func coordinatorHeartbeatRetryInterval(interval time.Duration) time.Duration {
	return max(interval/3, time.Millisecond)
}

// coordinatorHeartbeatStep decides when the next renewal runs. A claim the store rejects stops the
// heartbeat; any other failure retries inside the lease window so a transient error cannot let the
// claim expire and hand the run to a second attempt.
func coordinatorHeartbeatStep(interval time.Duration, err error) (time.Duration, bool) {
	switch {
	case coordinatorLeaseLost(err):
		return 0, true
	case err != nil:
		return coordinatorHeartbeatRetryInterval(interval), false
	default:
		return interval, false
	}
}
