package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestSchedulerLifecycleContract(t *testing.T) {
	t.Parallel()

	t.Run("Should stop the runtime loop when the start context is canceled", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 17, 16, 30, 0, 0, time.UTC)
		clock := clockwork.NewFakeClockAt(base)
		tickSeen := make(chan struct{}, 1)
		scheduler := newTestScheduler(
			t,
			&fakeTaskSource{recoverCh: tickSeen},
			&fakeSessionSource{},
			&fakeWaker{},
			WithClock(clock),
			WithInterval(time.Minute),
		)

		ctx, cancel := context.WithCancel(testutil.Context(t))
		if err := scheduler.Start(ctx); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		done := runtimeDoneForLifecycleTest(t, scheduler)
		waitForClockTimers(t, clock, 1)

		cancel()
		waitForRuntimeDone(t, done)
		clock.Advance(time.Minute)
		assertNoRecoveryTick(t, tickSeen)
	})

	t.Run("Should allow shutdown retry after the first shutdown deadline expires", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 5, 17, 16, 35, 0, 0, time.UTC)
		clock := clockwork.NewFakeClockAt(base)
		source := newBlockingRecoveryTaskSource()
		scheduler := newTestScheduler(
			t,
			source,
			&fakeSessionSource{},
			&fakeWaker{},
			WithClock(clock),
			WithInterval(time.Minute),
		)
		t.Cleanup(source.release)

		if err := scheduler.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		waitForClockTimers(t, clock, 1)
		clock.Advance(time.Minute)
		source.waitForRecovery(t)

		shutdownCtx, cancel := context.WithTimeout(testutil.Context(t), time.Nanosecond)
		defer cancel()
		err := scheduler.Shutdown(shutdownCtx)
		if err == nil {
			t.Fatal("Shutdown(expired context) error = nil, want deadline error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(expired context) error = %v, want deadline exceeded", err)
		}

		retryDone := make(chan error, 1)
		go func() {
			retryDone <- scheduler.Shutdown(testutil.Context(t))
		}()
		assertShutdownStillWaiting(t, retryDone)

		source.release()
		select {
		case err := <-retryDone:
			if err != nil {
				t.Fatalf("Shutdown(retry) error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for shutdown retry")
		}
	})
}

func runtimeDoneForLifecycleTest(t *testing.T, scheduler *Scheduler) <-chan struct{} {
	t.Helper()

	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if scheduler.runtimeDone == nil {
		t.Fatal("scheduler runtimeDone = nil, want active runtime")
	}
	return scheduler.runtimeDone
}

func waitForRuntimeDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scheduler runtime to stop")
	}
}

func assertNoRecoveryTick(t *testing.T, tickSeen <-chan struct{}) {
	t.Helper()

	select {
	case <-tickSeen:
		t.Fatal("scheduler recovered after start context cancellation")
	case <-time.After(50 * time.Millisecond):
	}
}

func assertShutdownStillWaiting(t *testing.T, retryDone <-chan error) {
	t.Helper()

	select {
	case err := <-retryDone:
		t.Fatalf("Shutdown(retry) returned before runtime stopped: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

type blockingRecoveryTaskSource struct {
	entered     chan struct{}
	releaseCh   chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingRecoveryTaskSource() *blockingRecoveryTaskSource {
	return &blockingRecoveryTaskSource{
		entered:   make(chan struct{}),
		releaseCh: make(chan struct{}),
	}
}

func (s *blockingRecoveryTaskSource) PendingRuns(context.Context) ([]RunSnapshot, error) {
	return nil, nil
}

func (s *blockingRecoveryTaskSource) ActiveRuns(context.Context) ([]taskpkg.Run, error) {
	return nil, nil
}

func (s *blockingRecoveryTaskSource) GetRunStatus(
	context.Context,
	string,
) (taskpkg.RunStatus, bool, error) {
	return taskpkg.TaskRunStatusUnknown, false, nil
}

func (s *blockingRecoveryTaskSource) RecoverExpiredRunLeases(
	context.Context,
	taskpkg.ExpiredLeaseRecovery,
	taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	s.enteredOnce.Do(func() {
		close(s.entered)
	})
	<-s.releaseCh
	return nil, nil
}

func (s *blockingRecoveryTaskSource) ExpireTaskBlocks(
	context.Context,
	time.Time,
	taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return taskpkg.ExpireTaskBlocksResult{}, nil
}

func (s *blockingRecoveryTaskSource) waitForRecovery(t *testing.T) {
	t.Helper()

	select {
	case <-s.entered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for recovery call")
	}
}

func (s *blockingRecoveryTaskSource) release() {
	s.releaseOnce.Do(func() {
		close(s.releaseCh)
	})
}
