package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestInfoValidateRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info Info
	}{
		{
			name: "missing pid",
			info: Info{
				StartedAt: time.Now().UTC(),
				State:     ReadyStateReady,
			},
		},
		{
			name: "invalid port",
			info: Info{
				PID:       1,
				HTTPPort:  70000,
				StartedAt: time.Now().UTC(),
				State:     ReadyStateReady,
			},
		},
		{
			name: "missing started at",
			info: Info{
				PID:   1,
				State: ReadyStateReady,
			},
		},
		{
			name: "missing state",
			info: Info{
				PID:       1,
				StartedAt: time.Now().UTC(),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.info.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
		})
	}
}

func TestInfoFileLifecycle(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon", "daemon.json")
	info := Info{
		PID:        42,
		Version:    "test",
		SocketPath: "/tmp/daemon.sock",
		StartedAt:  time.Unix(100, 0).UTC(),
		State:      ReadyStateReady,
	}

	if err := WriteInfo(path, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	loaded, err := ReadInfo(path)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if loaded != info {
		t.Fatalf("loaded info = %#v, want %#v", loaded, info)
	}

	if err := RemoveInfo(path); err != nil {
		t.Fatalf("RemoveInfo() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat(%s) error = %v, want os.ErrNotExist", path, err)
	}
	if err := RemoveInfo(path); err != nil {
		t.Fatalf("RemoveInfo(missing) error = %v", err)
	}
}

func TestReadInfoRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid daemon info: %v", err)
	}

	if _, err := ReadInfo(path); err == nil {
		t.Fatal("ReadInfo() error = nil, want non-nil")
	}
}

func TestWriteInfoRejectsInvalidInfo(t *testing.T) {
	t.Parallel()

	if err := WriteInfo(filepath.Join(t.TempDir(), "daemon.json"), Info{}); err == nil {
		t.Fatal("WriteInfo() error = nil, want non-nil")
	}
}

func TestInfoHelpersRejectEmptyPaths(t *testing.T) {
	t.Parallel()

	if _, err := ReadInfo(" "); err == nil {
		t.Fatal("ReadInfo(empty) error = nil, want non-nil")
	}
	if err := WriteInfo(" ", Info{}); err == nil {
		t.Fatal("WriteInfo(empty) error = nil, want non-nil")
	}
	if err := RemoveInfo(" "); err != nil {
		t.Fatalf("RemoveInfo(empty) error = %v, want nil", err)
	}
}

func TestAcquireLockLifecycle(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.lock")
	lock, err := AcquireLock(path, os.Getpid())
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	if got := lock.Path(); got != path {
		t.Fatalf("lock.Path() = %q, want %q", got, path)
	}
	if got := lock.StalePID(); got != 0 {
		t.Fatalf("lock.StalePID() = %d, want 0", got)
	}

	pid, err := readLockPID(path)
	if err != nil {
		t.Fatalf("readLockPID() error = %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("readLockPID() = %d, want %d", pid, os.Getpid())
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("lock.Release() error = %v", err)
	}
	pid, err = readLockPID(path)
	if err != nil {
		t.Fatalf("readLockPID(after release) error = %v", err)
	}
	if pid != 0 {
		t.Fatalf("readLockPID(after release) = %d, want 0", pid)
	}
}

func TestAcquireLockDetectsStalePID(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("999001\n"), 0o600); err != nil {
		t.Fatalf("write stale pid: %v", err)
	}

	lock, err := acquireLock(path, os.Getpid(), lockDeps{
		newFlock: func(lockPath string) *flock.Flock { return flock.New(lockPath) },
		processAlive: func(pid int) bool {
			return pid == os.Getpid()
		},
	})
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}
	defer func() {
		_ = lock.Release()
	}()

	if got := lock.StalePID(); got != 999001 {
		t.Fatalf("lock.StalePID() = %d, want 999001", got)
	}
}

func TestReadLockPIDHandlesInvalidContents(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatalf("write invalid lock pid: %v", err)
	}

	pid, err := readLockPID(path)
	if err != nil {
		t.Fatalf("readLockPID() error = %v", err)
	}
	if pid != 0 {
		t.Fatalf("readLockPID() = %d, want 0", pid)
	}
}

func TestReadLockPIDMissingFileReturnsZero(t *testing.T) {
	t.Parallel()

	pid, err := readLockPID(filepath.Join(t.TempDir(), "missing.lock"))
	if err != nil {
		t.Fatalf("readLockPID(missing) error = %v", err)
	}
	if pid != 0 {
		t.Fatalf("readLockPID(missing) = %d, want 0", pid)
	}
}

func TestErrAlreadyRunningHelpers(t *testing.T) {
	t.Parallel()

	err := errAlreadyRunning{pid: 77}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("errors.Is(err, ErrAlreadyRunning) = false")
	}
	if got := err.Error(); got == "" {
		t.Fatal("Error() returned empty string")
	}

	var generic errAlreadyRunning
	if got := generic.Error(); got != ErrAlreadyRunning.Error() {
		t.Fatalf("generic Error() = %q, want %q", got, ErrAlreadyRunning.Error())
	}
}

func TestAcquireLockRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	if _, err := AcquireLock(" ", os.Getpid()); err == nil {
		t.Fatal("AcquireLock(empty path) error = nil, want non-nil")
	}
	if _, err := AcquireLock(filepath.Join(t.TempDir(), "daemon.lock"), 0); err == nil {
		t.Fatal("AcquireLock(invalid pid) error = nil, want non-nil")
	}
}

func TestHostAccessorsAndCloseRemovesOwnedInfo(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.LockPath), 0o700); err != nil {
		t.Fatalf("mkdir daemon dir: %v", err)
	}

	info := Info{
		PID:        os.Getpid(),
		Version:    "test",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(200, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	host := &Host{
		paths: paths,
		lock:  &Lock{path: paths.LockPath, pid: info.PID},
		info:  info,
	}
	if got := host.Paths(); got.HomeDir != paths.HomeDir {
		t.Fatalf("host.Paths().HomeDir = %q, want %q", got.HomeDir, paths.HomeDir)
	}
	if got := host.Info(); got.PID != info.PID {
		t.Fatalf("host.Info().PID = %d, want %d", got.PID, info.PID)
	}

	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("host.Close() error = %v", err)
	}
	if _, err := os.Stat(paths.InfoPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat(%s) error = %v, want os.ErrNotExist", paths.InfoPath, err)
	}
}

func TestReleaseNilLock(t *testing.T) {
	t.Parallel()

	var lock *Lock
	if err := lock.Release(); err != nil {
		t.Fatalf("nil lock Release() error = %v", err)
	}
}

func TestNilLockHelpersReturnZeroValues(t *testing.T) {
	t.Parallel()

	var lock *Lock
	if got := lock.Path(); got != "" {
		t.Fatalf("lock.Path() = %q, want empty", got)
	}
	if got := lock.StalePID(); got != 0 {
		t.Fatalf("lock.StalePID() = %d, want 0", got)
	}
}

func TestProcessAliveRejectsZeroPID(t *testing.T) {
	t.Parallel()

	if ProcessAlive(0) {
		t.Fatal("ProcessAlive(0) = true, want false")
	}
}
