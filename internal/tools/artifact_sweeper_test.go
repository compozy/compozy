package tools

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestToolArtifactSweeperLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should sweep periodically and join shutdown", func(t *testing.T) {
		t.Parallel()

		store := &sweeperTestArtifactStore{swept: make(chan struct{}, 1)}
		worker := NewToolArtifactSweeper(store, time.Millisecond, nil)
		if err := worker.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		select {
		case <-store.swept:
		case <-time.After(time.Second):
			t.Fatal("periodic Sweep() was not observed")
		}
		if err := worker.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		before := store.sweepCount()
		time.Sleep(3 * time.Millisecond)
		if after := store.sweepCount(); after != before {
			t.Fatalf("Sweep calls after shutdown = %d, want %d", after, before)
		}
	})
}

type sweeperTestArtifactStore struct {
	mu    sync.Mutex
	count int
	swept chan struct{}
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
	select {
	case s.swept <- struct{}{}:
	default:
	}
	return nil
}

func (s *sweeperTestArtifactStore) sweepCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}
