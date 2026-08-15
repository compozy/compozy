package attachments

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/retention"
)

func TestSweeperLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should sweep periodically and join shutdown", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			const interval = time.Hour

			store := &sweeperTestStore{}
			worker := NewSweeper(store, interval, nil)
			if err := worker.Start(t.Context()); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			t.Cleanup(func() {
				if err := worker.Shutdown(context.WithoutCancel(t.Context())); err != nil {
					t.Errorf("cleanup Shutdown() error = %v", err)
				}
			})

			synctest.Wait()
			if got := store.sweepCount(); got != 0 {
				t.Fatalf("Sweep calls before the interval = %d, want 0", got)
			}

			time.Sleep(interval)
			synctest.Wait()
			if got := store.sweepCount(); got != 1 {
				t.Fatalf("Sweep calls after one interval = %d, want 1", got)
			}

			if err := worker.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			before := store.sweepCount()
			time.Sleep(3 * interval)
			synctest.Wait()
			if after := store.sweepCount(); after != before {
				t.Fatalf("Sweep calls after shutdown = %d, want %d", after, before)
			}
		})
	})

	t.Run("Should reject invalid start state and duplicate starts", func(t *testing.T) {
		t.Parallel()

		if err := NewSweeper(nil, time.Hour, nil).Start(t.Context()); !errors.Is(err, ErrAttachmentSweeperStoreRequired) {
			t.Fatalf("Start(nil store) error = %v, want store-required error", err)
		}
		store := &sweeperTestStore{}
		if err := NewSweeper(store, 0, nil).Start(t.Context()); !errors.Is(err, retention.ErrPeriodicWorkerInvalidInterval) {
			t.Fatalf("Start(zero interval) error = %v, want interval validation error", err)
		}
		worker := NewSweeper(store, time.Hour, nil)
		if err := worker.Start(nil); err == nil || //nolint:staticcheck // exercises the nil-context guard
			!errors.Is(err, retention.ErrPeriodicWorkerContextRequired) {
			t.Fatalf("Start(nil context) error = %v, want context-required error", err)
		}
		if err := worker.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := worker.Shutdown(context.WithoutCancel(t.Context())); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		if err := worker.Start(t.Context()); !errors.Is(err, retention.ErrPeriodicWorkerAlreadyStarted) {
			t.Fatalf("Start(duplicate) error = %v, want already-started error", err)
		}
	})

	t.Run("Should report sweep errors and allow idempotent shutdown", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			const interval = time.Hour

			store := &sweeperTestStore{err: ErrPersistence}
			errCh := make(chan error, 1)
			worker := NewSweeper(store, interval, func(err error) {
				errCh <- err
			})
			if err := worker.Start(t.Context()); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			time.Sleep(interval)
			synctest.Wait()
			select {
			case err := <-errCh:
				if !errors.Is(err, ErrPersistence) {
					t.Fatalf("onError() error = %v, want persistence error", err)
				}
			default:
				t.Fatal("onError() was not called")
			}
			if err := worker.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			if err := worker.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown(idempotent) error = %v", err)
			}
		})
	})

	t.Run("Should retain worker ownership after a timed-out shutdown", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{}, 1)
		release := make(chan struct{})
		store := &sweeperTestStore{started: started, release: release}
		worker := NewSweeper(store, time.Millisecond, nil)
		if err := worker.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for blocking sweep")
		}

		shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		defer cancel()
		if err := worker.Shutdown(shutdownCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(timeout) error = %v, want deadline exceeded", err)
		}
		if err := worker.Start(t.Context()); !errors.Is(err, retention.ErrPeriodicWorkerAlreadyStarted) {
			t.Fatalf("Start(after timeout) error = %v, want already-started error", err)
		}

		close(release)
		if err := worker.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown(join) error = %v", err)
		}
		if err := worker.Start(t.Context()); err != nil {
			t.Fatalf("Start(after join) error = %v", err)
		}
		if err := worker.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown(restarted) error = %v", err)
		}
	})
}

type sweeperTestStore struct {
	mu      sync.Mutex
	count   int
	err     error
	started chan<- struct{}
	release <-chan struct{}
}

var _ AttachmentSweeper = (*sweeperTestStore)(nil)

func (s *sweeperTestStore) Sweep(context.Context) error {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		<-s.release
	}
	s.mu.Lock()
	s.count++
	err := s.err
	s.mu.Unlock()
	return err
}

func (s *sweeperTestStore) sweepCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}
