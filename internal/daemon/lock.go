package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofrs/flock"
)

var ErrAlreadyRunning = errors.New("daemon: already running")

type errAlreadyRunning struct {
	pid int
}

func (e errAlreadyRunning) Error() string {
	if e.pid > 0 {
		return fmt.Sprintf("daemon: already running with pid %d", e.pid)
	}
	return ErrAlreadyRunning.Error()
}

func (e errAlreadyRunning) Unwrap() error {
	return ErrAlreadyRunning
}

type lockDeps struct {
	newFlock     func(string) *flock.Flock
	processAlive func(int) bool
}

// Lock owns the singleton daemon file lock.
type Lock struct {
	flock    *flock.Flock
	path     string
	pid      int
	stalePID int
}

// AcquireLock acquires the singleton daemon lock and records the current PID in the lock file.
func AcquireLock(path string, pid int) (*Lock, error) {
	return acquireLock(path, pid, lockDeps{
		newFlock:     func(lockPath string) *flock.Flock { return flock.New(lockPath) },
		processAlive: ProcessAlive,
	})
}

func acquireLock(path string, pid int, deps lockDeps) (*Lock, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("daemon: lock path is required")
	}
	if pid <= 0 {
		return nil, fmt.Errorf("daemon: invalid daemon pid %d", pid)
	}
	if deps.newFlock == nil {
		deps.newFlock = func(lockPath string) *flock.Flock { return flock.New(lockPath) }
	}
	if deps.processAlive == nil {
		deps.processAlive = ProcessAlive
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("daemon: create lock directory for %q: %w", cleanPath, err)
	}

	priorPID, err := readLockPID(cleanPath)
	if err != nil {
		return nil, err
	}

	fileLock := deps.newFlock(cleanPath)
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("daemon: acquire daemon lock %q: %w", cleanPath, err)
	}
	if !locked {
		return nil, errAlreadyRunning{pid: priorPID}
	}

	stalePID := 0
	if priorPID > 0 && priorPID != pid && !deps.processAlive(priorPID) {
		stalePID = priorPID
	}

	if err := writeLockPID(cleanPath, pid); err != nil {
		unlockErr := fileLock.Unlock()
		closeErr := fileLock.Close()
		return nil, errors.Join(err, unlockErr, closeErr)
	}

	return &Lock{
		flock:    fileLock,
		path:     cleanPath,
		pid:      pid,
		stalePID: stalePID,
	}, nil
}

// Path reports the on-disk daemon lock path.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// StalePID reports the recovered stale daemon PID, if any.
func (l *Lock) StalePID() int {
	if l == nil {
		return 0
	}
	return l.stalePID
}

// Release clears the lock file contents and releases the advisory file lock.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}

	var errs []error
	if err := writeLockPID(l.path, 0); err != nil {
		errs = append(errs, err)
	}
	if l.flock != nil {
		if err := l.flock.Unlock(); err != nil {
			errs = append(errs, fmt.Errorf("daemon: unlock daemon lock %q: %w", l.path, err))
		}
		if err := l.flock.Close(); err != nil {
			errs = append(errs, fmt.Errorf("daemon: close daemon lock %q: %w", l.path, err))
		}
	}

	l.flock = nil
	l.stalePID = 0
	return errors.Join(errs...)
}

func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("daemon: read daemon lock %q: %w", path, err)
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return 0, nil
	}

	pid, err := strconv.Atoi(raw)
	if err != nil {
		return 0, nil
	}
	if pid <= 0 {
		return 0, nil
	}
	return pid, nil
}

func writeLockPID(path string, pid int) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("daemon: open daemon lock %q for write: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	if pid > 0 {
		if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
			return fmt.Errorf("daemon: write daemon lock %q: %w", path, err)
		}
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("daemon: sync daemon lock %q: %w", path, err)
	}
	return nil
}
