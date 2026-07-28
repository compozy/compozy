package memory

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConsolidationLockConcurrentReclaimContract(t *testing.T) {
	t.Parallel()

	t.Run("Should allow only one acquisition from a released lock file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), consolidationLockName)
		writeLockState(t, path, "", time.Now().UTC().Add(-time.Minute).Round(0))

		assertSingleConcurrentLockAcquire(t, path, nil)
	})

	t.Run("Should allow only one acquisition from a stale PID lock file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), consolidationLockName)
		writeLockState(t, path, "424242", time.Now().UTC().Add(-2*time.Hour).Round(0))

		assertSingleConcurrentLockAcquire(t, path, func(lock *ConsolidationLock) {
			lock.processAlive = func(int) bool { return false }
		})
	})

	t.Run("Should allow only one acquisition when an unlocked stale claim file remains", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), consolidationLockName)
		staleAt := time.Now().UTC().Add(-2 * time.Hour).Round(0)
		writeLockState(t, path, "", staleAt)
		claimPath := path + ".claim"
		if err := os.WriteFile(claimPath, []byte("stale"), lockFilePerm); err != nil {
			t.Fatalf("WriteFile(claimPath) error = %v", err)
		}
		if err := os.Chtimes(claimPath, staleAt, staleAt); err != nil {
			t.Fatalf("Chtimes(claimPath) error = %v", err)
		}

		assertSingleConcurrentLockAcquire(t, path, nil)
	})
}

func assertSingleConcurrentLockAcquire(
	t *testing.T,
	path string,
	configure func(*ConsolidationLock),
) {
	t.Helper()

	const workers = 32
	start := make(chan struct{})
	var wait sync.WaitGroup
	var successCount atomic.Int32
	errs := make(chan error, workers)
	for range workers {
		wait.Go(func() {
			lock := NewConsolidationLock(path)
			if configure != nil {
				configure(lock)
			}
			<-start
			if _, ok, err := lock.TryAcquire(); err != nil {
				errs <- err
			} else if ok {
				successCount.Add(1)
			}
		})
	}

	close(start)
	wait.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("TryAcquire() concurrent error = %v", err)
	}
	if got := successCount.Load(); got != 1 {
		t.Fatalf("successful acquisitions = %d, want 1", got)
	}
	if got, want := readLockBody(t, path), strconv.Itoa(os.Getpid()); got != want {
		t.Fatalf("lock body after concurrent acquire = %q, want %q", got, want)
	}
}
