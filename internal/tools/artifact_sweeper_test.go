package tools

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestToolArtifactSweeperLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should sweep periodically and join shutdown", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			const interval = time.Hour

			store := &sweeperTestArtifactStore{}
			worker := NewToolArtifactSweeper(store, interval, nil)
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
}

type sweeperTestArtifactStore struct {
	mu    sync.Mutex
	count int
}

var _ ToolArtifactStore = (*sweeperTestArtifactStore)(nil)

func (s *sweeperTestArtifactStore) Put(
	context.Context,
	string,
	[]byte,
) (ArtifactRef, error) {
	return ArtifactRef{}, ErrToolArtifactPersistence
}

func (s *sweeperTestArtifactStore) ReadPage(
	context.Context,
	string,
	string,
	int64,
	int64,
) (ToolArtifactPage, error) {
	return ToolArtifactPage{}, ErrToolArtifactNotFound
}

func (s *sweeperTestArtifactStore) Sweep(context.Context) error {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return nil
}

func (s *sweeperTestArtifactStore) sweepCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}
