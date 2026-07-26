package resources

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

type recordingReconcileEventSink struct {
	mu     sync.Mutex
	events []ReconcileEvent
}

func (s *recordingReconcileEventSink) ObserveReconcileEvent(_ context.Context, event ReconcileEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *recordingReconcileEventSink) count(eventType ReconcileEventType) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, event := range s.events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func (s *recordingReconcileEventSink) waitForCount(
	t *testing.T,
	eventType ReconcileEventType,
	want int,
) {
	t.Helper()

	waitForCondition(t, time.Second, func() bool {
		return s.count(eventType) >= want
	})
}

type recordingReconcileHealthSink struct {
	mu      sync.Mutex
	updates []ReconcileHealth
}

func (s *recordingReconcileHealthSink) ReportReconcileHealth(_ context.Context, health ReconcileHealth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, health)
}

func (s *recordingReconcileHealthSink) latest() ReconcileHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.updates) == 0 {
		return ReconcileHealth{}
	}
	return s.updates[len(s.updates)-1]
}

func (s *recordingReconcileHealthSink) waitForStatus(
	t *testing.T,
	status ReconcileHealthStatus,
) ReconcileHealth {
	t.Helper()

	var latest ReconcileHealth
	waitForCondition(t, time.Second, func() bool {
		latest = s.latest()
		return latest.Status == status
	})
	return latest
}

var errMissingListRawDeadline = errors.New("deadline checking raw store saw no deadline")

type deadlineCheckingRawStore struct {
	listCalls atomic.Int32
}

func (s *deadlineCheckingRawStore) PutRaw(context.Context, MutationActor, RawDraft) (RawRecord, error) {
	return RawRecord{}, errors.New("deadline checking raw store PutRaw should not be called")
}

func (s *deadlineCheckingRawStore) DeleteRaw(context.Context, MutationActor, ResourceKind, string, int64) error {
	return errors.New("deadline checking raw store DeleteRaw should not be called")
}

func (s *deadlineCheckingRawStore) ApplySourceSnapshotRaw(context.Context, MutationActor, SourceSnapshot) error {
	return errors.New("deadline checking raw store ApplySourceSnapshotRaw should not be called")
}

func (s *deadlineCheckingRawStore) GetRaw(context.Context, MutationActor, ResourceKind, string) (RawRecord, error) {
	return RawRecord{}, errors.New("deadline checking raw store GetRaw should not be called")
}

func (s *deadlineCheckingRawStore) ListRaw(
	ctx context.Context,
	_ MutationActor,
	_ ResourceFilter,
) ([]RawRecord, error) {
	s.listCalls.Add(1)
	if _, ok := ctx.Deadline(); !ok {
		return nil, errMissingListRawDeadline
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

type reentrantTriggerSink struct {
	driver   ReconcileDriver
	resultCh chan error
	started  atomic.Bool
}

func (s *reentrantTriggerSink) ObserveReconcileEvent(_ context.Context, event ReconcileEvent) {
	if event.Type != ReconcileEventRequested || event.Kind != testResourceKind {
		return
	}

	if !s.started.CompareAndSwap(false, true) {
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- s.driver.Trigger(context.Background(), testResourceKind, ReconcileReasonWrite)
	}()

	select {
	case err := <-done:
		s.resultCh <- err
	case <-time.After(100 * time.Millisecond):
		s.resultCh <- errors.New("reentrant trigger timed out")
	}
}

func newTestProjectorRegistration(
	kind ResourceKind,
	dependsOn []ResourceKind,
	build func(context.Context, projectionInput) (ProjectionPlan, error),
	apply func(context.Context, ProjectionPlan) error,
) ProjectorRegistration {
	return &projectorRegistration{
		kind:      kind.Normalize(),
		dependsOn: normalizeKinds(dependsOn),
		build:     build,
		apply:     apply,
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func waitForReconcileIdle(t *testing.T, driver ReconcileDriver, kind ResourceKind) {
	t.Helper()

	impl, ok := driver.(*reconcileDriver)
	if !ok {
		t.Fatalf("driver type = %T, want *reconcileDriver", driver)
	}
	normalizedKind := kind.Normalize()
	waitForCondition(t, time.Second, func() bool {
		impl.mu.Lock()
		defer impl.mu.Unlock()

		if len(impl.queue) != 0 {
			return false
		}
		state := impl.kindStates[normalizedKind]
		return state != nil && !state.pending && !state.running && !state.dirty
	})
}

func TestReconcileDriverSingleFlightCoalescesSameKind(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce concurrent writes for one kind", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		eventSink := &recordingReconcileEventSink{}

		var mu sync.Mutex
		buildCalls := 0
		firstBuildStarted := make(chan struct{})
		releaseFirstBuild := make(chan struct{})

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(_ context.Context, _ projectionInput) (ProjectionPlan, error) {
						mu.Lock()
						buildCalls++
						call := buildCalls
						mu.Unlock()

						if call == 1 {
							close(firstBuildStarted)
							<-releaseFirstBuild
						}
						return testPlan{kind: testResourceKind, revision: int64(call), operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileEventSink(eventSink),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(first) error = %v", err)
		}
		<-firstBuildStarted

		for i := range 4 {
			if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
				t.Fatalf("Trigger(coalesced %d) error = %v", i, err)
			}
		}

		close(releaseFirstBuild)

		eventSink.waitForCount(t, ReconcileEventApplied, 2)
		waitForReconcileIdle(t, driver, testResourceKind)

		mu.Lock()
		gotBuildCalls := buildCalls
		mu.Unlock()
		if gotBuildCalls != 2 {
			t.Fatalf("buildCalls = %d, want 2", gotBuildCalls)
		}
		if got, wantMin := eventSink.count(ReconcileEventCoalesced), 1; got < wantMin {
			t.Fatalf("coalesced events = %d, want at least %d", got, wantMin)
		}
	})
}

func TestReconcileDriverPropagatesTimeoutToProjectorContexts(t *testing.T) {
	t.Parallel()

	t.Run("Should pass configured timeout to projector build contexts", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		timeout := 30 * time.Millisecond

		var mu sync.Mutex
		var capturedDeadline time.Time
		var buildErr error

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(ctx context.Context, _ projectionInput) (ProjectionPlan, error) {
						deadline, ok := ctx.Deadline()
						if !ok {
							t.Fatal("Build() context missing deadline")
						}

						mu.Lock()
						capturedDeadline = deadline
						mu.Unlock()

						<-ctx.Done()
						buildErr = ctx.Err()
						return nil, ctx.Err()
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileTimeout(timeout),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		startedAt := time.Now()
		err = driver.RunBoot(testutil.Context(t))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RunBoot() error = %v, want context.DeadlineExceeded", err)
		}
		if !errors.Is(buildErr, context.DeadlineExceeded) {
			t.Fatalf("Build() error = %v, want context.DeadlineExceeded", buildErr)
		}

		mu.Lock()
		deadline := capturedDeadline
		mu.Unlock()
		if deadline.IsZero() {
			t.Fatal("captured deadline was not set")
		}

		remaining := deadline.Sub(startedAt)
		if remaining <= 0 || remaining > timeout+50*time.Millisecond {
			t.Fatalf("deadline offset = %s, want within (0,%s]", remaining, timeout+50*time.Millisecond)
		}
	})

	t.Run("Should bound raw input loading with configured timeout", func(t *testing.T) {
		t.Parallel()

		rawStore := &deadlineCheckingRawStore{}
		driver, err := NewReconcileDriver(
			rawStore,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						return testPlan{kind: testResourceKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileTimeout(20*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		err = driver.RunBoot(context.Background())
		if errors.Is(err, errMissingListRawDeadline) {
			t.Fatalf("RunBoot() error = %v, raw input loading was not deadline-bound", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RunBoot() error = %v, want context.DeadlineExceeded", err)
		}
		if got, want := rawStore.listCalls.Load(), int32(1); got != want {
			t.Fatalf("ListRaw() calls = %d, want %d", got, want)
		}
	})
}

func TestReconcileDriverOpensDegradedCircuitAndWaitsForFreshWrite(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh a pending dirty rerun after degraded failure", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		eventSink := &recordingReconcileEventSink{}
		healthSink := &recordingReconcileHealthSink{}

		var mu sync.Mutex
		buildCalls := 0
		firstBuildStarted := make(chan struct{})
		releaseFirstBuild := make(chan struct{})

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(_ context.Context, _ projectionInput) (ProjectionPlan, error) {
						mu.Lock()
						buildCalls++
						call := buildCalls
						mu.Unlock()

						if call == 1 {
							close(firstBuildStarted)
							<-releaseFirstBuild
						}
						return nil, errors.New("boom")
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileCoalesceWindow(20*time.Millisecond),
			WithReconcileFailureThreshold(1),
			WithReconcileDegradedBackoff(time.Hour),
			WithReconcileEventSink(eventSink),
			WithReconcileHealthSink(healthSink),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(first) error = %v", err)
		}
		<-firstBuildStarted
		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(coalesced) error = %v", err)
		}

		close(releaseFirstBuild)
		eventSink.waitForCount(t, ReconcileEventDegraded, 1)
		healthSink.waitForStatus(t, ReconcileHealthStatusDegraded)

		mu.Lock()
		gotBuildCalls := buildCalls
		mu.Unlock()
		if gotBuildCalls != 1 {
			t.Fatalf("buildCalls after degraded failure = %d, want 1", gotBuildCalls)
		}
		if got := eventSink.count(ReconcileEventDegraded); got != 1 {
			t.Fatalf("degraded events = %d, want 1", got)
		}
		if got := healthSink.latest().Status; got != ReconcileHealthStatusDegraded {
			t.Fatalf("latest health status = %q, want %q", got, ReconcileHealthStatusDegraded)
		}

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(fresh write) error = %v", err)
		}

		waitForCondition(t, time.Second, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return buildCalls == 2
		})
	})

	t.Run("Should bypass degraded backoff for fresh writes after a completed failure", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		healthSink := &recordingReconcileHealthSink{}
		var buildCalls atomic.Int32

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						buildCalls.Add(1)
						return nil, errors.New("boom")
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileFailureThreshold(1),
			WithReconcileDegradedBackoff(time.Hour),
			WithReconcileHealthSink(healthSink),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(first) error = %v", err)
		}
		healthSink.waitForStatus(t, ReconcileHealthStatusDegraded)
		if got, want := buildCalls.Load(), int32(1); got != want {
			t.Fatalf("buildCalls after degraded failure = %d, want %d", got, want)
		}

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(fresh write) error = %v", err)
		}

		waitForCondition(t, time.Second, func() bool {
			return buildCalls.Load() == 2
		})
	})
}

func TestReconcileDriverSchedulesReverseDependenciesAfterWritesOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should root write fans out to dependents in order", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)

		var mu sync.Mutex
		var order []ResourceKind
		recordOrder := func(kind ResourceKind) {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, kind)
		}

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(bundleKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						recordOrder(bundleKind)
						return testPlan{kind: bundleKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
				newTestProjectorRegistration(bundleActivationKind, []ResourceKind{bundleKind},
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						recordOrder(bundleActivationKind)
						return testPlan{kind: bundleActivationKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
				newTestProjectorRegistration(ResourceKind("automation.job"), nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						recordOrder(ResourceKind("automation.job"))
						return testPlan{kind: ResourceKind("automation.job"), revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, bundleKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(bundle) error = %v", err)
		}

		waitForCondition(t, time.Second, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(order) == 2
		})

		mu.Lock()
		gotOrder := append([]ResourceKind(nil), order...)
		mu.Unlock()
		wantOrder := []ResourceKind{bundleKind, bundleActivationKind}
		if len(gotOrder) != len(wantOrder) {
			t.Fatalf("order = %#v, want %#v", gotOrder, wantOrder)
		}
		for idx := range wantOrder {
			if gotOrder[idx] != wantOrder[idx] {
				t.Fatalf("order[%d] = %q, want %q (full=%#v)", idx, gotOrder[idx], wantOrder[idx], gotOrder)
			}
		}
	})

	t.Run("Should dependent write does not reverse-trigger dependencies", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)

		var mu sync.Mutex
		var order []ResourceKind
		recordOrder := func(kind ResourceKind) {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, kind)
		}

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(bundleKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						recordOrder(bundleKind)
						return testPlan{kind: bundleKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
				newTestProjectorRegistration(bundleActivationKind, []ResourceKind{bundleKind},
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						recordOrder(bundleActivationKind)
						return testPlan{kind: bundleActivationKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, bundleActivationKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(bundle.activation) error = %v", err)
		}

		waitForCondition(t, time.Second, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(order) == 1
		})

		mu.Lock()
		gotOrder := append([]ResourceKind(nil), order...)
		mu.Unlock()
		if len(gotOrder) != 1 || gotOrder[0] != bundleActivationKind {
			t.Fatalf("order = %#v, want only %q", gotOrder, bundleActivationKind)
		}
	})

	t.Run("Should dependency-only write fans out to registered dependents", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		buildResult := make(chan error, 1)

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(bundleActivationKind, []ResourceKind{bundleKind},
					func(_ context.Context, input projectionInput) (ProjectionPlan, error) {
						if _, ok := input.dependencies[bundleKind]; !ok {
							buildResult <- errors.New("bundle dependency records were not loaded")
							return nil, errors.New("bundle dependency records were not loaded")
						}
						buildResult <- nil
						return testPlan{kind: bundleActivationKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, bundleKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger(bundle dependency-only write) error = %v", err)
		}

		select {
		case err := <-buildResult:
			if err != nil {
				t.Fatalf("Build() dependency input error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for dependency-only write fan-out")
		}
	})
}

func TestReconcileDriverValidationAndLifecycleErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should registered projectors require raw store", func(t *testing.T) {
		t.Parallel()

		_, err := NewReconcileDriver(
			nil,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						return testPlan{kind: testResourceKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
		)
		if err == nil {
			t.Fatal("NewReconcileDriver() error = nil, want raw-store validation failure")
		}
	})

	t.Run("Should closed and unknown kinds are rejected", func(t *testing.T) {
		t.Parallel()

		driver, err := NewReconcileDriver(nil, MutationActor{}, nil)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}

		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := driver.Close(closeCtx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		if err := driver.Trigger(testutil.Context(t), testResourceKind, ReconcileReasonWrite); err == nil {
			t.Fatal("Trigger(closed) error = nil, want closed-driver failure")
		}
	})

	t.Run("Should reason validation rejects unsupported values", func(t *testing.T) {
		t.Parallel()

		if err := ReconcileReason("invalid").Validate("reason"); !errors.Is(err, ErrValidation) {
			t.Fatalf("ReconcileReason.Validate() error = %v, want ErrValidation", err)
		}
	})
}

func TestReconcileDriverEventSinkCanReenterTrigger(t *testing.T) {
	t.Parallel()

	t.Run("Should allow event sink to reenter trigger without deadlock", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		sink := &reentrantTriggerSink{resultCh: make(chan error, 1)}

		driver, err := NewReconcileDriver(
			kernel,
			testDaemonActor(),
			[]ProjectorRegistration{
				newTestProjectorRegistration(testResourceKind, nil,
					func(context.Context, projectionInput) (ProjectionPlan, error) {
						return testPlan{kind: testResourceKind, revision: 1, operations: 1}, nil
					},
					func(context.Context, ProjectionPlan) error { return nil },
				),
			},
			WithReconcileEventSink(sink),
		)
		if err != nil {
			t.Fatalf("NewReconcileDriver() error = %v", err)
		}
		sink.driver = driver
		t.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if closeErr := driver.Close(closeCtx); closeErr != nil {
				t.Fatalf("Close() error = %v", closeErr)
			}
		})

		if err := driver.Trigger(ctx, testResourceKind, ReconcileReasonWrite); err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}

		select {
		case err := <-sink.resultCh:
			if err != nil {
				t.Fatalf("reentrant trigger result = %v, want nil", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for reentrant trigger result")
		}
	})
}
