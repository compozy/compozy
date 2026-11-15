package task

import (
	"context"
	"errors"
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
		interrupted := make(chan int, 1)
		go func() {
			<-started
			interrupted <- harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID, "operator request")
		}()

		settled, err := harness.start(runner)
		if !errors.Is(err, ErrCoordinatorInterrupted) {
			t.Fatalf("StartRun(interrupted coordinator) error = %v, want ErrCoordinatorInterrupted", err)
		}
		if got := <-interrupted; got != 1 {
			t.Fatalf("InterruptLoopCoordinatorRuns() = %d, want 1", got)
		}
		if settled == nil || settled.Status.Normalize() != TaskRunStatusFailed {
			t.Fatalf("StartRun(interrupted coordinator) run = %#v, want failed settlement", settled)
		}
		if got := harness.manager.InterruptLoopCoordinatorRuns(harness.loopRunID, "again"); got != 0 {
			t.Fatalf("InterruptLoopCoordinatorRuns(after settlement) = %d, want 0", got)
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
		if got := harness.store.progressLoopRunIDs(); len(got) != 0 {
			t.Fatalf("loop progress advances = %#v, want none after the claim was lost", got)
		}
	})
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

// progressRecordingManagerStore adds the Loop progress capability the daemon's SQLite store owns.
type progressRecordingManagerStore struct {
	*inMemoryManagerStore
	progressMu sync.Mutex
	progress   []coordinatorProgress
}

var _ CoordinatorProgressRecorder = (*progressRecordingManagerStore)(nil)

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
	return h.manager.StartRun(h.t.Context(), claim.Run.ID, StartRun{
		ClaimToken:     claim.ClaimToken,
		IdempotencyKey: claim.Run.IdempotencyKey,
	}, h.actor)
}
