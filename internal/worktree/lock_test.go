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
		acquired := make(chan func(), 1)
		go func() {
			release, acquireErr := locks.Acquire(context.Background(), commonDir)
			if acquireErr == nil {
				acquired <- release
			}
		}()
		select {
		case release := <-acquired:
			release()
			t.Fatal("second caller acquired before the first released")
		default:
		}
		releaseFirst()
		releaseSecond := <-acquired
		releaseSecond()
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
