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

func TestConsolidationLockTryAcquireFreshFileWritesPID(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)

	priorMtime, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatal("TryAcquire() ok = false, want true")
	}
	if !priorMtime.IsZero() {
		t.Fatalf("TryAcquire() priorMtime = %v, want zero", priorMtime)
	}
	if got := readLockBody(t, path); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock body = %q, want current pid", got)
	}
}

func TestConsolidationLockTryAcquireFailsWhenLiveProcessHoldsLock(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	lock.processAlive = func(int) bool { return true }
	writeLockState(t, path, strconv.Itoa(os.Getpid()), time.Now().UTC())

	_, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if ok {
		t.Fatal("TryAcquire() ok = true, want false")
	}
}

func TestConsolidationLockTryAcquireRejectsFreshCurrentPIDWhenLivenessProbeFails(t *testing.T) {
	t.Parallel()

	t.Run("Should reject fresh current PID ownership even when liveness probe fails", func(t *testing.T) {
		t.Parallel()

		lock, path := newTestLock(t)
		now := time.Now().UTC().Round(0)
		lock.now = func() time.Time { return now }
		lock.processAlive = func(int) bool { return false }
		writeLockState(t, path, strconv.Itoa(os.Getpid()), now.Add(-time.Minute))

		_, ok, err := lock.TryAcquire()
		if err != nil {
			t.Fatalf("TryAcquire() error = %v", err)
		}
		if ok {
			t.Fatal("TryAcquire() ok = true, want false")
		}
		if got := readLockBody(t, path); got != strconv.Itoa(os.Getpid()) {
			t.Fatalf("lock body = %q, want current pid", got)
		}
	})
}

func TestConsolidationLockTryAcquireReclaimsDeadPID(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	lock.processAlive = func(int) bool { return false }
	prior := time.Now().UTC().Add(-10 * time.Minute).Round(0)
	writeLockState(t, path, "424242", prior)

	priorMtime, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatal("TryAcquire() ok = false, want true")
	}
	if !priorMtime.Equal(prior) {
		t.Fatalf("TryAcquire() priorMtime = %v, want %v", priorMtime, prior)
	}
	if got := readLockBody(t, path); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock body = %q, want current pid", got)
	}
}

func TestConsolidationLockTryAcquireReclaimsOldLock(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	lock.processAlive = func(int) bool { return true }
	prior := time.Now().UTC().Add(-2 * time.Hour).Round(0)
	writeLockState(t, path, strconv.Itoa(os.Getpid()), prior)

	priorMtime, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatal("TryAcquire() ok = false, want true")
	}
	if !priorMtime.Equal(prior) {
		t.Fatalf("TryAcquire() priorMtime = %v, want %v", priorMtime, prior)
	}
}

func TestConsolidationLockReleaseClearsPIDAndUpdatesMtime(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	releasedAt := time.Now().UTC().Add(5 * time.Minute).Round(0)
	lock.now = func() time.Time { return releasedAt }
	if _, ok, err := lock.TryAcquire(); err != nil || !ok {
		t.Fatalf("TryAcquire() = (%v, %v, %v), want ok acquisition", time.Time{}, ok, err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if got := readLockBody(t, path); got != "" {
		t.Fatalf("lock body after release = %q, want empty", got)
	}

	lastConsolidatedAt, err := lock.LastConsolidatedAt()
	if err != nil {
		t.Fatalf("LastConsolidatedAt() error = %v", err)
	}
	if !lastConsolidatedAt.Equal(releasedAt) {
		t.Fatalf("LastConsolidatedAt() = %v, want %v", lastConsolidatedAt, releasedAt)
	}
}

func TestConsolidationLockRollbackRestoresPriorMtime(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	prior := time.Now().UTC().Add(-6 * time.Hour).Round(0)
	writeLockState(t, path, "", prior)

	priorMtime, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatal("TryAcquire() ok = false, want true")
	}
	if !priorMtime.Equal(prior) {
		t.Fatalf("TryAcquire() priorMtime = %v, want %v", priorMtime, prior)
	}

	if err := lock.Rollback(priorMtime); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if got := readLockBody(t, path); got != "" {
		t.Fatalf("lock body after rollback = %q, want empty", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if !info.ModTime().Equal(prior) {
		t.Fatalf("lock mtime after rollback = %v, want %v", info.ModTime(), prior)
	}
}

func TestConsolidationLockLastConsolidatedAtReadsMtime(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	want := time.Now().UTC().Add(-12 * time.Hour).Round(0)
	writeLockState(t, path, "", want)

	got, err := lock.LastConsolidatedAt()
	if err != nil {
		t.Fatalf("LastConsolidatedAt() error = %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("LastConsolidatedAt() = %v, want %v", got, want)
	}
}

func TestConsolidationLockLastConsolidatedAtMissingFileReturnsZero(t *testing.T) {
	t.Parallel()

	lock, _ := newTestLock(t)

	got, err := lock.LastConsolidatedAt()
	if err != nil {
		t.Fatalf("LastConsolidatedAt() error = %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("LastConsolidatedAt() = %v, want zero time", got)
	}
}

func TestConsolidationLockTryAcquireAllowsOnlyOneConcurrentWriter(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), consolidationLockName)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for range 2 {
		wg.Go(func() {
			lock := NewConsolidationLock(path)
			<-start
			if _, ok, err := lock.TryAcquire(); err == nil && ok {
				successCount.Add(1)
			}
		})
	}

	close(start)
	wg.Wait()

	if got := successCount.Load(); got != 1 {
		t.Fatalf("successful acquisitions = %d, want 1", got)
	}
}

func TestConsolidationLockTryAcquireRestoresPriorStateWhenPIDInvalid(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	prior := time.Now().UTC().Add(-8 * time.Hour).Round(0)
	writeLockState(t, path, "", prior)
	lock.pidFn = func() int { return 0 }

	priorMtime, ok, err := lock.TryAcquire()
	if err == nil {
		t.Fatal("TryAcquire() error = nil, want non-nil")
	}
	if ok {
		t.Fatal("TryAcquire() ok = true, want false")
	}
	if !priorMtime.Equal(prior) {
		t.Fatalf("TryAcquire() priorMtime = %v, want %v", priorMtime, prior)
	}
	if got := readLockBody(t, path); got != "" {
		t.Fatalf("lock body after failed acquire = %q, want empty", got)
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("os.Stat() error = %v", statErr)
	}
	if !info.ModTime().Equal(prior) {
		t.Fatalf("lock mtime after failed acquire = %v, want %v", info.ModTime(), prior)
	}
}

func TestConsolidationLockTryAcquireReclaimsCorruptBody(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	prior := time.Now().UTC().Add(-3 * time.Hour).Round(0)
	writeLockState(t, path, "not-a-pid", prior)

	_, ok, err := lock.TryAcquire()
	if err != nil {
		t.Fatalf("TryAcquire() error = %v", err)
	}
	if !ok {
		t.Fatal("TryAcquire() ok = false, want true")
	}
	if got := readLockBody(t, path); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock body = %q, want current pid", got)
	}
}

func TestConsolidationLockRollbackRemovesFileWhenPriorZero(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)
	if _, ok, err := lock.TryAcquire(); err != nil || !ok {
		t.Fatalf("TryAcquire() = (%v, %v, %v), want ok acquisition", time.Time{}, ok, err)
	}

	if err := lock.Rollback(time.Time{}); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("os.Stat() error = %v, want os.ErrNotExist", err)
	}
}

func TestConsolidationLockValidateRequiresPath(t *testing.T) {
	t.Parallel()

	lock := NewConsolidationLock("")
	if _, err := lock.LastConsolidatedAt(); err == nil {
		t.Fatal("LastConsolidatedAt() error = nil, want non-nil")
	}
}

func TestConsolidationLockReleaseFailsWhenParentIsFile(t *testing.T) {
	t.Parallel()

	parentFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), filePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	lock := NewConsolidationLock(filepath.Join(parentFile, consolidationLockName))
	if err := lock.Release(); err == nil {
		t.Fatal("Release() error = nil, want non-nil")
	}
}

func TestConsolidationLockTryAcquireFailsWhenParentIsFile(t *testing.T) {
	t.Parallel()

	parentFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), filePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	lock := NewConsolidationLock(filepath.Join(parentFile, consolidationLockName))
	if _, ok, err := lock.TryAcquire(); err == nil || ok {
		t.Fatalf("TryAcquire() = (_, %v, %v), want error and ok=false", ok, err)
	}
}

func TestConsolidationLockValidateNilReceiver(t *testing.T) {
	t.Parallel()

	var lock *ConsolidationLock
	if err := lock.Release(); err == nil {
		t.Fatal("Release() error = nil, want non-nil")
	}
}

func TestConsolidationLockRestoreUnlocked(t *testing.T) {
	t.Parallel()

	lock, path := newTestLock(t)

	if err := lock.restoreUnlocked(time.Time{}); err != nil {
		t.Fatalf("restoreUnlocked(zero) error = %v", err)
	}

	prior := time.Now().UTC().Add(-4 * time.Hour).Round(0)
	if err := lock.restoreUnlocked(prior); err != nil {
		t.Fatalf("restoreUnlocked(prior) error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if !info.ModTime().Equal(prior) {
		t.Fatalf("lock mtime after restoreUnlocked() = %v, want %v", info.ModTime(), prior)
	}
}

func newTestLock(t *testing.T) (*ConsolidationLock, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), consolidationLockName)
	return NewConsolidationLock(path), path
}

func writeLockState(t *testing.T, path string, body string, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(body), lockFilePerm); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("os.Chtimes() error = %v", err)
	}
}

func readLockBody(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	return string(data)
}
