package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrCoordinatorInterrupted identifies coordinator work stopped because its Loop run left the live set.
var ErrCoordinatorInterrupted = errors.New("task: coordinator run interrupted")

// errCoordinatorLeaseLost identifies coordinator work stopped because its claim no longer holds the run.
var errCoordinatorLeaseLost = errors.New("task: coordinator lease lost")

// LoopRunLivenessReader reports whether a Loop run still accepts coordinator work. The task manager
// consumes it as an optional store capability, so a manager without a Loop-owning store keeps its
// in-process behavior.
type LoopRunLivenessReader interface {
	CoordinatedLoopRunLive(ctx context.Context, loopRunID string) (bool, error)
}

type coordinatorExecutionContextKey struct{}

// CoordinatorIdentity identifies one claimed evaluation without exposing its bearer token.
type CoordinatorIdentity struct {
	RunID        string
	LoopRunID    string
	EvaluationID string
}

// CoordinatorExecution returns the current evaluation's process ownership identity.
func CoordinatorExecution(ctx context.Context) (CoordinatorIdentity, bool) {
	identity, ok := ctx.Value(coordinatorExecutionContextKey{}).(CoordinatorIdentity)
	return identity, ok
}

type inFlightCoordinator struct {
	loopRunID string
	claimHash string
	cancel    context.CancelCauseFunc
	done      <-chan struct{}
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
	key, unregister := m.registerInFlightCoordinator(run, cancel)
	execCtx = context.WithValue(execCtx, coordinatorExecutionContextKey{}, CoordinatorIdentity{
		RunID: run.ID, LoopRunID: run.LoopRunID, EvaluationID: fmt.Sprintf("%s:%d", key.runID, key.seq),
	})
	var confirmedDeadline atomic.Int64
	confirmedDeadline.Store(run.LeaseUntil.UnixNano())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		m.watchCoordinatorLease(execCtx, cancel, run, &confirmedDeadline)
	}()
	defer func() {
		cancel(nil)
		<-watchDone
	}()
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
	now := m.now().UTC()
	firstDue := now.Add(coordinatorFirstHeartbeatDelay(
		coordinatorHeartbeatInterval(coordinatorLeaseDuration(run)), run.LeaseUntil, now,
	))
	heartbeatDone := make(chan error, 1)
	go func() {
		heartbeatDone <- m.heartbeatCoordinatorLease(execCtx, cancel, run, claimToken, actor, &confirmedDeadline, firstDue)
	}()
	plan, runErr := m.evaluateClaimedCoordinator(execCtx, key, run, claimToken)
	unregister()
	// An interruption or a lost claim invalidates whatever the runner computed under it.
	interruption := context.Cause(execCtx)
	cancel(nil)
	heartbeatErr := <-heartbeatDone
	if runErr == nil && heartbeatErr == nil {
		// Completion fences transactionally as well; transient reads do not fail a healthy gate.
		if err := m.checkCoordinatorClaim(ctx, run, claimToken); coordinatorLeaseLost(err) {
			heartbeatErr = err
		}
	}
	return coordinatorEvaluationOutcome(plan, runErr, interruption, heartbeatErr)
}

func (m *Service) evaluateClaimedCoordinator(
	ctx context.Context, key coordinatorEvaluationKey, run Run, claimToken string,
) (CoordinatorCompletionPlan, error) {
	// The heartbeat already runs before synchronous ownership admission.
	if err := m.checkCoordinatorClaim(ctx, run, claimToken); err != nil {
		return CoordinatorCompletionPlan{}, err
	}
	if err := m.retireSupersededCoordinators(ctx, key, run); err != nil {
		return CoordinatorCompletionPlan{}, err
	}
	if err := context.Cause(ctx); err != nil {
		return CoordinatorCompletionPlan{}, err
	}
	return m.coordinatorRunner.Run(ctx, RunID(run.ID))
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

func (m *Service) registerInFlightCoordinator(
	run Run, cancel context.CancelCauseFunc,
) (coordinatorEvaluationKey, func()) {
	runID := strings.TrimSpace(run.ID)
	m.coordinatorMu.Lock()
	if m.inFlightCoordinators == nil {
		m.inFlightCoordinators = make(map[coordinatorEvaluationKey]inFlightCoordinator)
	}
	m.coordinatorSeq++
	key := coordinatorEvaluationKey{runID: runID, seq: m.coordinatorSeq}
	done := make(chan struct{})
	m.inFlightCoordinators[key] = inFlightCoordinator{
		loopRunID: strings.TrimSpace(run.LoopRunID), claimHash: run.ClaimTokenHash,
		cancel: cancel, done: done,
	}
	m.coordinatorMu.Unlock()
	// Unregistering deletes only this evaluation's key. A replacement registered for the same run
	// ID while this evaluation finishes therefore stays reachable for interruption.
	return key, sync.OnceFunc(func() {
		m.coordinatorMu.Lock()
		delete(m.inFlightCoordinators, key)
		close(done)
		m.coordinatorMu.Unlock()
	})
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
	confirmedDeadline *atomic.Int64,
	firstDue time.Time,
) error {
	leaseDuration := coordinatorLeaseDuration(run)
	interval := coordinatorHeartbeatInterval(leaseDuration)
	confirmedUntil := run.LeaseUntil
	firstDelay := max(firstDue.Sub(m.now().UTC()), time.Millisecond)
	poll := m.coordinatorPollInterval(interval, firstDelay)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	next := firstDue
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := m.checkCoordinatorClaim(ctx, run, claimToken)
			if err == nil && !m.loopRunAcceptsCoordinator(ctx, run.LoopRunID) {
				err = ErrCoordinatorInterrupted
			}
			if coordinatorLeaseLost(err) {
				leaseErr := fmt.Errorf("%w: coordinator run %q: %w", errCoordinatorLeaseLost, run.ID, err)
				cancel(leaseErr)
				return leaseErr
			}
			if errors.Is(err, ErrCoordinatorInterrupted) {
				cancel(err)
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			now := m.now().UTC()
			if err == nil && !now.Before(next) {
				renewed, renewErr := m.HeartbeatRunLease(ctx, LeaseHeartbeat{
					RunID: run.ID, ClaimToken: claimToken, LeaseDuration: leaseDuration, Now: now,
				}, actor)
				err = renewErr
				if coordinatorLeaseLost(err) {
					leaseErr := fmt.Errorf("%w: coordinator run %q: %w", errCoordinatorLeaseLost, run.ID, err)
					cancel(leaseErr)
					return leaseErr
				}
				if ctx.Err() != nil {
					return nil
				}
				delay, _ := coordinatorHeartbeatStep(interval, err)
				next = now.Add(delay)
				if err == nil {
					confirmedUntil = renewed.LeaseUntil
					confirmedDeadline.Store(confirmedUntil.UnixNano())
					poll = m.coordinatorPollInterval(interval, interval)
					ticker.Reset(poll)
				}
			}
			if err != nil {
				slog.Warn("task: observe or renew coordinator lease", "run_id", run.ID, "error", err)
			}
			// Generic storage failures must not leave shell work alive beyond the last
			// confirmed claim. Re-read the clock after I/O, which may have stalled.
			if !m.now().UTC().Add(poll).Before(confirmedUntil) {
				leaseErr := fmt.Errorf("%w: %w", errCoordinatorLeaseLost, errors.Join(ErrLeaseExpired, err))
				cancel(leaseErr)
				return leaseErr
			}
		}
	}
}

func (m *Service) checkCoordinatorClaim(ctx context.Context, run Run, token string) error {
	current, err := m.store.GetTaskRun(ctx, run.ID)
	if err != nil {
		return err
	}
	if !VerifyClaimToken(token, current.ClaimTokenHash) || current.Status != TaskRunStatusRunning {
		return ErrInvalidClaimToken
	}
	if !current.LeaseUntil.After(m.now().UTC()) {
		return ErrLeaseExpired
	}
	return nil
}

func coordinatorLeaseLost(err error) bool {
	return errors.Is(err, ErrInvalidClaimToken) ||
		errors.Is(err, ErrLeaseExpired) ||
		errors.Is(err, ErrInvalidStatusTransition) ||
		errors.Is(err, ErrTaskRunNotFound)
}

func coordinatorLeaseDuration(run Run) time.Duration {
	since := run.HeartbeatAt
	if since.IsZero() {
		since = run.ClaimedAt
	}
	if !since.IsZero() && run.LeaseUntil.After(since) {
		return min(run.LeaseUntil.Sub(since), MaxRunLeaseDuration)
	}
	return DefaultRunLeaseDuration
}

func coordinatorHeartbeatInterval(leaseDuration time.Duration) time.Duration {
	return max(leaseDuration/3, time.Millisecond)
}

// coordinatorFirstHeartbeatDelay keeps the first renewal inside the claimed lease. The transitions
// before execution can consume part of the lease before the heartbeat starts, so the
// first wait uses at most one third of what remains; later renewals keep the steady cadence.
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
	return max(min(remaining/3, interval), minimumDelay)
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

// watchCoordinatorLease is independent of storage admission. A stalled read or write
// cannot prevent execution cancellation at the last confirmed ownership deadline.
func (m *Service) watchCoordinatorLease(
	ctx context.Context, cancel context.CancelCauseFunc, run Run, confirmed *atomic.Int64,
) {
	interval := coordinatorHeartbeatInterval(coordinatorLeaseDuration(run))
	first := coordinatorFirstHeartbeatDelay(interval, run.LeaseUntil, m.now().UTC())
	poll := m.coordinatorPollInterval(interval, first)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.now().UTC().Add(poll).Before(time.Unix(0, confirmed.Load())) {
				cancel(fmt.Errorf("%w: %w", errCoordinatorLeaseLost, ErrLeaseExpired))
				return
			}
			poll = m.coordinatorPollInterval(interval, time.Unix(0, confirmed.Load()).Sub(m.now().UTC())/3)
			ticker.Reset(poll)
		}
	}
}

func (m *Service) coordinatorPollInterval(interval, firstDelay time.Duration) time.Duration {
	ceiling := m.coordinatorPollMax
	if ceiling <= 0 {
		ceiling = time.Second
	}
	return max(min(interval/3, ceiling, firstDelay), time.Millisecond)
}

// Called only after verifying this evaluation's claim. Capture exact older keys,
// never interrupt by the shared RunID: a late old caller must not hit a replacement.
func (m *Service) retireSupersededCoordinators(ctx context.Context, current coordinatorEvaluationKey, run Run) error {
	var pending []<-chan struct{}
	m.coordinatorMu.Lock()
	for key, evaluation := range m.inFlightCoordinators {
		if key.runID != current.runID || key.seq >= current.seq || evaluation.claimHash == run.ClaimTokenHash {
			continue
		}
		evaluation.cancel(fmt.Errorf("%w: %w", errCoordinatorLeaseLost, ErrInvalidClaimToken))
		pending = append(pending, evaluation.done)
	}
	m.coordinatorMu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	return nil
}
