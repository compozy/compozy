package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Invariant: a claimed coordinator keeps its lease for as long as its evaluation runs, and stops
// when its Loop run is canceled or its claim is lost, so a long gate command is never dispatched
// twice and a superseded evaluation never runs to completion as an orphan.
func TestManagerCoordinatorLivenessShouldHoldTheClaimWhileEvaluating(t *testing.T) {
	t.Parallel()

	t.Run("Should renew the claim past its original lease so no second attempt is dispatched", func(t *testing.T) {
		t.Parallel()

		harness := newCoordinatorLivenessHarness(t, "renew")
		var recoveredInFlight []ExpiredLeaseRecoveryResult
		var reclaimErr error
		calls := 0
		runner := coordinatorRunnerFunc(func(ctx context.Context, _ RunID) (CoordinatorCompletionPlan, error) {
			calls++
			for range 3 {
				harness.clock.advance(harness.lease * 2 / 3)
				harness.store.waitForProgressAt(t, harness.clock.now())
			}
			var err error
			recoveredInFlight, err = harness.manager.RecoverExpiredRunLeases(ctx, ExpiredLeaseRecovery{
				Now:    harness.clock.now(),
				Reason: "worker_crashed",
			}, harness.actor)
			if err != nil {
				return CoordinatorCompletionPlan{}, err
			}
			_, reclaimErr = harness.manager.ClaimNextRun(ctx, harness.claimCriteria(harness.clock.now()), harness.actor)
			return CoordinatorCompletionPlan{Yield: true}, nil
		})

		completed, err := harness.start(runner)
		if err != nil {
			t.Fatalf("StartRun(long coordinator) error = %v", err)
		}
		if got, want := completed.Status.Normalize(), TaskRunStatusCompleted; got != want {
			t.Fatalf("StartRun(long coordinator).Status = %q, want %q", got, want)
		}
		if calls != 1 {
			t.Fatalf("coordinator runner calls = %d, want exactly one", calls)
		}
		if len(recoveredInFlight) != 0 {
			t.Fatalf("expired lease recovery during evaluation = %#v, want none", recoveredInFlight)
		}
		if !errors.Is(reclaimErr, ErrActiveRunLease) {
			t.Fatalf("ClaimNextRun(during evaluation) error = %v, want ErrActiveRunLease", reclaimErr)
		}
		if got := harness.store.progressLoopRunIDs(); len(got) < 3 || got[0] != harness.loopRunID {
			t.Fatalf("loop progress advances = %#v, want at least three for %q", got, harness.loopRunID)
		}
	})

	t.Run("Should advance Loop progress while the gate evaluation is still running", func(t *testing.T) {
		t.Parallel()

		harness := newCoordinatorLivenessHarness(t, "progress")
		started := make(chan struct{})
		release := make(chan struct{})
		runner := coordinatorRunnerFunc(func(ctx context.Context, _ RunID) (CoordinatorCompletionPlan, error) {
			close(started)
			select {
			case <-release:
				return CoordinatorCompletionPlan{Yield: true}, nil
			case <-ctx.Done():
				return CoordinatorCompletionPlan{}, ctx.Err()
			}
		})
		harness.prepare(runner)

		settled := make(chan error, 1)
		go func() {
			_, err := harness.startRun(harness.t.Context())
			settled <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("coordinator evaluation did not start")
		}
		// The Loop run must look alive before the evaluation settles, not only after it writes a verdict.
		harness.store.waitForProgressAt(t, harness.clock.now())
		progress := harness.store.progressLoopRunIDs()
		if len(progress) == 0 {
			t.Fatal("loop progress advances while running = 0 records, want at least one")
		}
		// A renewal can land before the evaluation settles, so extra matching records are valid.
		for index, loopRunID := range progress {
			if loopRunID != harness.loopRunID {
				t.Fatalf("loop progress record %d = %q, want %q", index, loopRunID, harness.loopRunID)
			}
		}

		close(release)
		if err := <-settled; err != nil {
			t.Fatalf("StartRun(evaluating coordinator) error = %v", err)
		}
	})

	t.Run("Should interrupt the evaluation and discard its plan when the Loop run is canceled", func(t *testing.T) {
		t.Parallel()

		harness := newCoordinatorLivenessHarness(t, "interrupt")
		started := make(chan struct{})
		runner := coordinatorRunnerFunc(func(ctx context.Context, _ RunID) (CoordinatorCompletionPlan, error) {
			close(started)
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
				t.Error("interrupted coordinator evaluation did not observe cancellation")
			}
			// A runner that ignores its context must still not commit a plan.
			return CoordinatorCompletionPlan{Yield: true}, nil
		})
		interrupted := make(chan [2]int, 1)
		go func() {
			<-started
			otherRun := harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID+"-other", "operator request")
			matchingRun := harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID, "operator request")
			interrupted <- [2]int{otherRun, matchingRun}
		}()

		settled, err := harness.start(runner)
		if !errors.Is(err, ErrCoordinatorInterrupted) {
			t.Fatalf("StartRun(interrupted coordinator) error = %v, want ErrCoordinatorInterrupted", err)
		}
		if got, want := <-interrupted, [2]int{0, 1}; got != want {
			t.Fatalf("InterruptLoopCoordinatorRuns(other, matching) = %v, want %v", got, want)
		}
		if settled == nil || settled.Status.Normalize() != TaskRunStatusFailed {
			t.Fatalf("StartRun(interrupted coordinator) run = %#v, want failed settlement", settled)
		}
		if got := harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID, "again"); got != 0 {
			t.Fatalf("InterruptLoopCoordinatorRuns(after settlement) = %d, want 0", got)
		}
	})

	t.Run("Should not start an evaluation whose Loop run was canceled before it registered", func(t *testing.T) {
		t.Parallel()

		harness := newCoordinatorLivenessHarness(t, "canceled-before-register")
		harness.store.setLoopRunLive(harness.loopRunID, false)
		calls := 0
		runner := coordinatorRunnerFunc(func(context.Context, RunID) (CoordinatorCompletionPlan, error) {
			calls++
			return CoordinatorCompletionPlan{Yield: true}, nil
		})

		settled, err := harness.start(runner)
		if !errors.Is(err, ErrCoordinatorInterrupted) {
			t.Fatalf("StartRun(canceled before registration) error = %v, want ErrCoordinatorInterrupted", err)
		}
		if calls != 0 {
			t.Fatalf("coordinator runner calls = %d, want none for a canceled Loop run", calls)
		}
		if settled == nil || settled.Status.Normalize() != TaskRunStatusFailed {
			t.Fatalf("StartRun(canceled before registration) run = %#v, want failed settlement", settled)
		}
	})

	t.Run("Should stop a superseded evaluation when its claim is lost", func(t *testing.T) {
		t.Parallel()

		harness := newCoordinatorLivenessHarness(t, "lost")
		runner := coordinatorRunnerFunc(func(ctx context.Context, _ RunID) (CoordinatorCompletionPlan, error) {
			harness.clock.advance(harness.lease * 2)
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
				t.Error("superseded coordinator evaluation was not interrupted")
			}
			return CoordinatorCompletionPlan{Yield: true}, nil
		})

		_, err := harness.start(runner)
		if !errors.Is(err, errCoordinatorLeaseLost) || !errors.Is(err, ErrLeaseExpired) {
			t.Fatalf("StartRun(superseded coordinator) error = %v, want lost expired lease", err)
		}
		if got := harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID, "late"); got != 0 {
			t.Fatalf("InterruptLoopCoordinatorRuns(superseded) = %d, want 0", got)
		}
	})
}

// Invariant: a transient renewal failure retries inside the current lease window, so a busy store
// cannot let the coordinator claim expire and hand the same run to a second attempt.
func TestCoordinatorHeartbeatStepShouldKeepRetriesInsideTheLease(t *testing.T) {
	t.Parallel()

	lease := DefaultRunLeaseDuration
	interval := coordinatorHeartbeatInterval(lease)
	tests := []struct {
		name      string
		err       error
		wantDelay time.Duration
		wantStop  bool
	}{
		{
			name:      "Should wait one heartbeat interval after a healthy renewal",
			wantDelay: interval,
		},
		{
			name:      "Should retry a transient store failure before the claim expires",
			err:       errors.New("store: database is locked"),
			wantDelay: coordinatorHeartbeatRetryInterval(interval),
		},
		{
			name:     "Should stop when the claim token no longer matches",
			err:      fmt.Errorf("%w: token mismatch", ErrInvalidClaimToken),
			wantStop: true,
		},
		{
			name:     "Should stop when the lease already expired",
			err:      fmt.Errorf("%w: claim taken over", ErrLeaseExpired),
			wantStop: true,
		},
		{
			name:     "Should stop when the run can no longer transition",
			err:      fmt.Errorf("%w: run is not running", ErrInvalidStatusTransition),
			wantStop: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			delay, stop := coordinatorHeartbeatStep(interval, tt.err)
			if stop != tt.wantStop {
				t.Fatalf("coordinatorHeartbeatStep(%v) stop = %t, want %t", tt.err, stop, tt.wantStop)
			}
			if delay != tt.wantDelay {
				t.Fatalf("coordinatorHeartbeatStep(%v) delay = %s, want %s", tt.err, delay, tt.wantDelay)
			}
		})
	}

	// Two failed renewals still land the next attempt inside the lease. Retrying after a full
	// interval instead would schedule the third attempt exactly when the claim expires.
	retryDelay := coordinatorHeartbeatRetryInterval(interval)
	if elapsed := interval + 2*retryDelay; elapsed >= lease {
		t.Fatalf("two retries finish after %s, want before the %s lease expires", elapsed, lease)
	}
	if fullIntervalRetries := interval + 2*interval; fullIntervalRetries < lease {
		t.Fatalf("full-interval retries finish after %s, want at least one due at the %s expiry",
			fullIntervalRetries, lease)
	}
}

// Invariant: every coordinator evaluation unregisters only itself, so a replacement registered for
// the same run ID after a recovered lease requeue stays reachable by interruption, and canceling a
// Loop interrupts every evaluation it still has in flight.
func TestManagerCoordinatorInFlightRegistryShouldKeepReplacementsReachable(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the replacement registered when the superseded evaluation finishes", func(t *testing.T) {
		t.Parallel()

		manager := &Service{}
		run := Run{ID: "run-registry-superseded", LoopRunID: "loop-run-registry-superseded"}
		var supersededCause, replacementCause error
		_, unregisterSuperseded := manager.registerInFlightCoordinator(run, func(cause error) {
			supersededCause = cause
		})
		_, unregisterReplacement := manager.registerInFlightCoordinator(run, func(cause error) {
			replacementCause = cause
		})

		// The superseded attempt returns after the replacement registered.
		unregisterSuperseded()
		if interrupted := manager.InterruptLoopCoordinatorRuns(run.LoopRunID, "operator cancel"); interrupted != 1 {
			t.Fatalf("InterruptLoopCoordinatorRuns(replacement only) = %d, want 1", interrupted)
		}
		if !errors.Is(replacementCause, ErrCoordinatorInterrupted) {
			t.Fatalf("replacement cancel cause = %v, want ErrCoordinatorInterrupted", replacementCause)
		}
		if supersededCause != nil {
			t.Fatalf("superseded cancel cause = %v, want it untouched after unregistering", supersededCause)
		}

		unregisterReplacement()
		if interrupted := manager.InterruptLoopCoordinatorRuns(run.LoopRunID, "again"); interrupted != 0 {
			t.Fatalf("InterruptLoopCoordinatorRuns(after settlement) = %d, want 0", interrupted)
		}
	})

	t.Run("Should interrupt both concurrent evaluations for the same Loop run", func(t *testing.T) {
		t.Parallel()

		manager := &Service{}
		run := Run{ID: "run-registry-concurrent", LoopRunID: "loop-run-registry-concurrent"}
		causes := make([]error, 2)
		unregister := make([]func(), 0, 2)
		for index := range causes {
			_, stop := manager.registerInFlightCoordinator(run, func(cause error) {
				causes[index] = cause
			})
			unregister = append(unregister, stop)
		}
		defer func() {
			for _, stop := range unregister {
				stop()
			}
		}()

		if interrupted := manager.InterruptLoopCoordinatorRuns(run.LoopRunID, "operator cancel"); interrupted != 2 {
			t.Fatalf("InterruptLoopCoordinatorRuns(concurrent evaluations) = %d, want 2", interrupted)
		}
		for index, cause := range causes {
			if !errors.Is(cause, ErrCoordinatorInterrupted) {
				t.Fatalf("evaluation %d cancel cause = %v, want ErrCoordinatorInterrupted", index, cause)
			}
		}
	})
}

// Invariant: a plan computed under a lost claim is discarded even when the evaluation returns before
// the heartbeat records the loss, and the first renewal respects what is left of the claimed lease.
func TestManagerCoordinatorLivenessShouldDiscardPlansFromLostClaims(t *testing.T) {
	t.Parallel()

	t.Run("Should discard the plan when only the heartbeat recorded the lease loss", func(t *testing.T) {
		t.Parallel()

		leaseErr := fmt.Errorf("%w: coordinator run %q: %w", errCoordinatorLeaseLost, "run-first", ErrLeaseExpired)
		plan, err := coordinatorEvaluationOutcome(
			CoordinatorCompletionPlan{Yield: true},
			nil,
			nil,
			leaseErr,
		)
		if !errors.Is(err, errCoordinatorLeaseLost) || !errors.Is(err, ErrLeaseExpired) {
			t.Fatalf("coordinatorEvaluationOutcome(heartbeat loss) error = %v, want the lease loss", err)
		}
		if plan.Yield || plan.Snapshot.LoopRunID != "" {
			t.Fatalf("coordinatorEvaluationOutcome(heartbeat loss) plan = %#v, want it discarded", plan)
		}
	})

	t.Run("Should join an interruption with the heartbeat lease loss", func(t *testing.T) {
		t.Parallel()

		interrupted := fmt.Errorf("%w: loop run canceled", ErrCoordinatorInterrupted)
		leaseErr := fmt.Errorf("%w: token mismatch", errCoordinatorLeaseLost)
		_, err := coordinatorEvaluationOutcome(CoordinatorCompletionPlan{}, nil, interrupted, leaseErr)
		if !errors.Is(err, ErrCoordinatorInterrupted) || !errors.Is(err, errCoordinatorLeaseLost) {
			t.Fatalf("coordinatorEvaluationOutcome(both causes) error = %v, want both causes", err)
		}
	})

	t.Run("Should return the plan when neither cause fired", func(t *testing.T) {
		t.Parallel()

		want := CoordinatorCompletionPlan{Yield: true}
		plan, err := coordinatorEvaluationOutcome(want, nil, nil, nil)
		if err != nil || !plan.Yield {
			t.Fatalf("coordinatorEvaluationOutcome(clean) = %#v, %v; want the plan", plan, err)
		}
	})
}

// Invariant: the first heartbeat lands inside the claimed lease instead of a full interval after the
// claim was made, so a claim whose start-up transitions consumed part of its lease still renews.
func TestManagerCoordinatorFirstHeartbeatShouldRespectTheRemainingLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	interval := coordinatorHeartbeatInterval(DefaultRunLeaseDuration)
	tests := []struct {
		name       string
		interval   time.Duration
		leaseUntil time.Time
		now        time.Time
		want       time.Duration
	}{
		{
			name:       "Should keep the steady interval when the full lease remains",
			interval:   interval,
			leaseUntil: now.Add(DefaultRunLeaseDuration),
			now:        now,
			want:       interval,
		},
		{
			name:       "Should renew before the remaining lease ends when startup was slow",
			interval:   interval,
			leaseUntil: now.Add(20 * time.Second),
			now:        now,
			want:       20 * time.Second / 3,
		},
		{
			name:       "Should fall back to the interval without a recorded lease end",
			interval:   interval,
			leaseUntil: time.Time{},
			now:        now,
			want:       interval,
		},
		{
			name:       "Should keep the timer positive for an expired lease",
			interval:   interval,
			leaseUntil: now.Add(-time.Minute),
			now:        now,
			want:       time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := coordinatorFirstHeartbeatDelay(tt.interval, tt.leaseUntil, tt.now); got != tt.want {
				t.Fatalf("coordinatorFirstHeartbeatDelay(%s) = %s, want %s",
					tt.leaseUntil.Sub(tt.now), got, tt.want)
			}
		})
	}
}

type coordinatorRunnerFunc func(context.Context, RunID) (CoordinatorCompletionPlan, error)

func (f coordinatorRunnerFunc) Run(ctx context.Context, runID RunID) (CoordinatorCompletionPlan, error) {
	return f(ctx, runID)
}

type coordinatorLivenessClock struct {
	mu      sync.Mutex
	current time.Time
}

func (c *coordinatorLivenessClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *coordinatorLivenessClock) advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(delta)
}

type coordinatorProgress struct {
	loopRunID string
	at        time.Time
}

// progressRecordingManagerStore adds the Loop progress and liveness capabilities the daemon's
// SQLite store owns.
type progressRecordingManagerStore struct {
	*inMemoryManagerStore
	progressMu sync.Mutex
	progress   []coordinatorProgress
	liveMu     sync.Mutex
	live       map[string]bool
}

var _ CoordinatorProgressRecorder = (*progressRecordingManagerStore)(nil)
var _ LoopRunLivenessReader = (*progressRecordingManagerStore)(nil)

// Match the production store's atomic coordinator heartbeat/progress mutation.
func (s *progressRecordingManagerStore) HeartbeatRunLease(ctx context.Context, h LeaseHeartbeat) (Run, error) {
	run, err := s.inMemoryManagerStore.HeartbeatRunLease(ctx, h)
	if err != nil {
		return Run{}, err
	}
	if err := s.AdvanceLoopRunProgress(ctx, run.LoopRunID, run.HeartbeatAt); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *progressRecordingManagerStore) AdvanceLoopRunProgress(
	_ context.Context,
	loopRunID string,
	at time.Time,
) error {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress = append(s.progress, coordinatorProgress{loopRunID: loopRunID, at: at})
	return nil
}

func (s *progressRecordingManagerStore) progressLoopRunIDs() []string {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	ids := make([]string, 0, len(s.progress))
	for _, progress := range s.progress {
		ids = append(ids, progress.loopRunID)
	}
	return ids
}

// CoordinatedLoopRunLive answers the optional Loop liveness capability. A run with no override stays
// live, matching the production store where the Loop run row decides.
func (s *progressRecordingManagerStore) CoordinatedLoopRunLive(_ context.Context, loopRunID string) (bool, error) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	live, overridden := s.live[loopRunID]
	if !overridden {
		return true, nil
	}
	return live, nil
}

func (s *progressRecordingManagerStore) setLoopRunLive(loopRunID string, live bool) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	if s.live == nil {
		s.live = make(map[string]bool)
	}
	s.live[loopRunID] = live
}

func (s *progressRecordingManagerStore) waitForProgressAt(t *testing.T, at time.Time) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.progressMu.Lock()
		reached := len(s.progress) > 0 && !s.progress[len(s.progress)-1].at.Before(at)
		s.progressMu.Unlock()
		if reached {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Errorf("coordinator lease was not renewed at %s", at)
}

type coordinatorLivenessHarness struct {
	t         *testing.T
	store     *progressRecordingManagerStore
	clock     *coordinatorLivenessClock
	actor     ActorContext
	lease     time.Duration
	loopRunID string
	runID     string
	manager   *Service
	claim     *ClaimResult
}

func newCoordinatorLivenessHarness(t *testing.T, suffix string) *coordinatorLivenessHarness {
	t.Helper()

	start := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	harness := &coordinatorLivenessHarness{
		t:         t,
		store:     &progressRecordingManagerStore{inMemoryManagerStore: newInMemoryManagerStore()},
		clock:     &coordinatorLivenessClock{current: start},
		lease:     90 * time.Millisecond,
		loopRunID: "loop-run-liveness-" + suffix,
		runID:     "run-loop-coordinator-liveness-" + suffix,
	}
	taskRecord := Task{
		ID: "task-loop-coordinator-liveness-" + suffix, Scope: ScopeGlobal, Title: "Loop coordinator liveness",
		Priority: PriorityMedium, MaxAttempts: DefaultTaskMaxAttempts, Status: TaskStatusReady,
		ApprovalPolicy: ApprovalPolicyNone, ApprovalState: ApprovalStateNotRequired,
		CreatedBy: ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
		Origin:    Origin{Kind: OriginKindDaemon, Ref: "loop"}, CreatedAt: start, UpdatedAt: start,
	}
	if err := harness.store.CreateTask(t.Context(), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := harness.store.CreateTaskRun(t.Context(), Run{
		ID: harness.runID, TaskID: taskRecord.ID, RunKind: RunKindCoordinator, LoopRunID: harness.loopRunID,
		Status: TaskRunStatusQueued, Attempt: 1, IdempotencyKey: "coordinator:liveness:" + suffix,
		Origin: Origin{Kind: OriginKindDaemon, Ref: "loop"}, QueuedAt: start,
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	actor, err := DeriveDaemonActorContext("loop", "daemon.loop")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	harness.actor = actor
	return harness
}

func (h *coordinatorLivenessHarness) claimCriteria(now time.Time) ClaimCriteria {
	return ClaimCriteria{
		RunID:            h.runID,
		Scope:            ScopeGlobal,
		RunKind:          RunKindCoordinator,
		ClaimerSessionID: "daemon-loop",
		ClaimedBy:        &ActorIdentity{Kind: ActorKindDaemon, Ref: "loop"},
		LeaseDuration:    h.lease,
		Now:              now,
	}
}

func (h *coordinatorLivenessHarness) start(runner CoordinatorRunner) (*Run, error) {
	h.t.Helper()

	h.prepare(runner)
	return h.startRun(h.t.Context())
}

func (h *coordinatorLivenessHarness) prepare(runner CoordinatorRunner) {
	h.t.Helper()

	h.manager = newTaskManagerForTestWithOptions(
		h.t,
		h.store,
		WithCoordinatorRunner(runner),
		WithGenerationStateFinalizer(noopLoopFinalizer{}),
		WithManagerNow(h.clock.now),
	)
	claim, err := h.manager.ClaimNextRun(h.t.Context(), h.claimCriteria(h.clock.now()), h.actor)
	if err != nil {
		h.t.Fatalf("ClaimNextRun() error = %v", err)
	}
	h.claim = claim
}

func (h *coordinatorLivenessHarness) startRun(ctx context.Context) (*Run, error) {
	return h.manager.StartRun(ctx, h.claim.Run.ID, StartRun{
		ClaimToken:     h.claim.ClaimToken,
		IdempotencyKey: h.claim.Run.IdempotencyKey,
	}, h.actor)
}
