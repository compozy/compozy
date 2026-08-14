package worktree

import (
	"context"
	"errors"
	"testing"
)

// Canonical suite: bounded per-repository mutation serialization.
func TestRepositoryLocks(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize callers for one canonical common directory", func(t *testing.T) {
		t.Parallel()
		commonDir := t.TempDir()
		locks := NewRepositoryLocks(1)
		releaseFirst, err := locks.Acquire(context.Background(), commonDir)
		if err != nil {
			t.Fatalf("Acquire(first) error = %v", err)
		}
		type acquireResult struct {
			release func()
			err     error
		}
		acquired := make(chan acquireResult, 1)
		go func() {
			release, acquireErr := locks.Acquire(context.Background(), commonDir)
			acquired <- acquireResult{release: release, err: acquireErr}
		}()
		select {
		case result := <-acquired:
			if result.release != nil {
				result.release()
			}
			t.Fatal("second caller acquired before the first released")
		default:
		}
		releaseFirst()
		result := <-acquired
		if result.err != nil {
			t.Fatalf("Acquire(second) error = %v", result.err)
		}
		result.release()
	})

	t.Run("Should refuse callers beyond the bounded waiter limit", func(t *testing.T) {
		t.Parallel()
		commonDir := t.TempDir()
		locks := NewRepositoryLocks(0)
		release, err := locks.Acquire(context.Background(), commonDir)
		if err != nil {
			t.Fatalf("Acquire(first) error = %v", err)
		}
		defer release()
		_, err = locks.Acquire(context.Background(), commonDir)
		if !errors.Is(err, ErrOperationInProgress) {
			t.Fatalf("Acquire(second) error = %v, want worktree_operation_in_progress", err)
		}
	})

	t.Run("Should release a canceled waiter without poisoning the repository lock", func(t *testing.T) {
		t.Parallel()
		commonDir := t.TempDir()
		locks := NewRepositoryLocks(1)
		releaseFirst, err := locks.Acquire(context.Background(), commonDir)
		if err != nil {
			t.Fatalf("Acquire(first) error = %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		canceled := make(chan error, 1)
		go func() {
			_, acquireErr := locks.Acquire(ctx, commonDir)
			canceled <- acquireErr
		}()
		cancel()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("Acquire(canceled waiter) error = %v, want context.Canceled", err)
		}
		releaseFirst()
		releaseNext, err := locks.Acquire(context.Background(), commonDir)
		if err != nil {
			t.Fatalf("Acquire(after cancel) error = %v", err)
		}
		releaseNext()
	})
}
