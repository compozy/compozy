package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/procutil"
	"github.com/gofrs/flock"
)

const (
	consolidationLockName = ".consolidate-lock"
	lockFilePerm          = 0o644
	defaultLockStaleAge   = time.Hour
)

// ConsolidationLock coordinates cross-process consolidation runs via a PID file
// whose mtime doubles as the last successful consolidation timestamp.
type ConsolidationLock struct {
	path         string
	staleAge     time.Duration
	pidFn        func() int
	now          func() time.Time
	processAlive func(int) bool
}

// NewConsolidationLock constructs a lock rooted at path with the default stale age.
func NewConsolidationLock(path string) *ConsolidationLock {
	return &ConsolidationLock{
		path:     strings.TrimSpace(path),
		staleAge: defaultLockStaleAge,
		pidFn:    os.Getpid,
		now: func() time.Time {
			return time.Now().UTC()
		},
		processAlive: procutil.Alive,
	}
}

// LastConsolidatedAt returns the lock file mtime, or the zero time when the lock file does not exist.
func (l *ConsolidationLock) LastConsolidatedAt() (time.Time, error) {
	if err := l.validate(); err != nil {
		return time.Time{}, err
	}

	info, err := os.Stat(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("memory: stat consolidation lock %q: %w", l.path, err)
	}

	return info.ModTime(), nil
}

// TryAcquire attempts to acquire the consolidation lock and returns the prior
// mtime for rollback when acquisition succeeds.
func (l *ConsolidationLock) TryAcquire() (priorMtime time.Time, acquired bool, err error) {
	if err := l.validate(); err != nil {
		return time.Time{}, false, err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return time.Time{}, false, fmt.Errorf("memory: create lock parent directory for %q: %w", l.path, err)
	}

	claim, claimed, err := l.tryAcquireMutationClaim()
	if err != nil {
		return time.Time{}, false, err
	}
	if !claimed {
		return time.Time{}, false, nil
	}
	claimHeld := true
	defer func() {
		if claimHeld {
			err = errors.Join(err, claim.release())
		}
	}()

	state, err := l.readState()
	if err != nil {
		return time.Time{}, false, err
	}
	if state.exists && state.validPID && !l.canReclaim(state) {
		return state.modTime, false, nil
	}

	priorMtime = state.modTime
	pid := l.pidFn()
	if pid <= 0 {
		return priorMtime, false, errors.Join(
			fmt.Errorf("memory: invalid process pid %d", pid),
		)
	}
	if state.exists && state.validPID && state.pid == pid && !l.isStale(state) {
		return state.modTime, false, nil
	}

	if err := l.writeLockedPID(pid); err != nil {
		return priorMtime, false, errors.Join(
			fmt.Errorf("memory: write consolidation lock %q: %w", l.path, err),
		)
	}

	if err := l.verifyLockedPID(pid, priorMtime); err != nil {
		return priorMtime, false, err
	}

	if releaseErr := claim.release(); releaseErr != nil {
		restoreErr := l.restoreUnlocked(priorMtime)
		return priorMtime, false, errors.Join(
			fmt.Errorf("memory: release consolidation lock claim %q: %w", l.claimPath(), releaseErr),
			restoreErr,
		)
	}
	claimHeld = false
	return priorMtime, true, nil
}

// Release clears the lock PID and leaves the file mtime at the release time.
func (l *ConsolidationLock) Release() error {
	if err := l.validate(); err != nil {
		return err
	}

	return l.withMutationClaim(func() error {
		if err := l.ensureOwnedByCurrentPID(); err != nil {
			return err
		}
		return l.writeUnlockedAt(l.now())
	})
}

// Rollback clears the lock PID and restores the mtime from before acquisition.
func (l *ConsolidationLock) Rollback(priorMtime time.Time) error {
	if err := l.validate(); err != nil {
		return err
	}

	return l.withMutationClaim(func() error {
		if err := l.ensureOwnedByCurrentPID(); err != nil {
			return err
		}
		if priorMtime.IsZero() {
			if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("memory: remove consolidation lock %q during rollback: %w", l.path, err)
			}
			return nil
		}

		return l.writeUnlockedAt(priorMtime)
	})
}

type lockState struct {
	exists   bool
	modTime  time.Time
	rawPID   string
	pid      int
	validPID bool
}

func (l *ConsolidationLock) validate() error {
	switch {
	case l == nil:
		return errors.New("memory: consolidation lock is required")
	case strings.TrimSpace(l.path) == "":
		return errors.New("memory: consolidation lock path is required")
	case l.pidFn == nil:
		return errors.New("memory: consolidation lock pid function is required")
	case l.now == nil:
		return errors.New("memory: consolidation lock clock is required")
	case l.processAlive == nil:
		return errors.New("memory: consolidation lock process liveness function is required")
	default:
		return nil
	}
}

func (l *ConsolidationLock) readState() (lockState, error) {
	info, err := os.Stat(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return lockState{}, nil
		}
		return lockState{}, fmt.Errorf("memory: stat consolidation lock %q: %w", l.path, err)
	}

	data, err := os.ReadFile(l.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return lockState{}, nil
		}
		return lockState{}, fmt.Errorf("memory: read consolidation lock %q: %w", l.path, err)
	}

	state := lockState{
		exists:  true,
		modTime: info.ModTime(),
		rawPID:  strings.TrimSpace(string(data)),
	}
	if state.rawPID == "" {
		return state, nil
	}

	pid, err := strconv.Atoi(state.rawPID)
	if err != nil {
		return state, nil
	}

	state.pid = pid
	state.validPID = pid > 0
	return state, nil
}

func (l *ConsolidationLock) canReclaim(state lockState) bool {
	if !state.validPID {
		return true
	}
	if l.isStale(state) {
		return true
	}
	return !l.processAlive(state.pid)
}

func (l *ConsolidationLock) isStale(state lockState) bool {
	return l.staleAge > 0 && l.now().Sub(state.modTime) > l.staleAge
}

func (l *ConsolidationLock) claimPath() string {
	return l.path + ".claim"
}

type lockMutationClaim struct {
	path string
	lock *flock.Flock
}

func (l *ConsolidationLock) tryAcquireMutationClaim() (*lockMutationClaim, bool, error) {
	claimPath := l.claimPath()
	fileLock := flock.New(claimPath)
	locked, err := fileLock.TryLock()
	if err != nil {
		closeErr := fileLock.Close()
		return nil, false, errors.Join(
			fmt.Errorf("memory: acquire consolidation lock claim %q: %w", claimPath, err),
			closeErr,
		)
	}
	if !locked {
		closeErr := fileLock.Close()
		if closeErr != nil {
			return nil, false, fmt.Errorf(
				"memory: close unacquired consolidation lock claim %q: %w",
				claimPath,
				closeErr,
			)
		}
		return nil, false, nil
	}
	return &lockMutationClaim{path: claimPath, lock: fileLock}, true, nil
}

func (c *lockMutationClaim) release() error {
	if c == nil || c.lock == nil {
		return nil
	}
	return errors.Join(
		wrapLockClaimError("unlock", c.path, c.lock.Unlock()),
		wrapLockClaimError("close", c.path, c.lock.Close()),
	)
}

func wrapLockClaimError(operation string, path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("memory: %s consolidation lock claim %q: %w", operation, path, err)
}

func (l *ConsolidationLock) withMutationClaim(fn func() error) (result error) {
	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return fmt.Errorf("memory: create lock parent directory for %q: %w", l.path, err)
	}
	claim, claimed, err := l.tryAcquireMutationClaim()
	if err != nil {
		return err
	}
	if !claimed {
		return ErrLockUnavailable
	}
	defer func() {
		result = errors.Join(result, claim.release())
	}()
	result = fn()
	return result
}

func (l *ConsolidationLock) ensureOwnedByCurrentPID() error {
	state, err := l.readState()
	if err != nil {
		return err
	}
	pid := l.pidFn()
	if pid <= 0 {
		return fmt.Errorf("memory: invalid process pid %d", pid)
	}
	if !state.exists || !state.validPID || state.pid != pid {
		return fmt.Errorf("memory: consolidation lock %q is not owned by pid %d", l.path, pid)
	}
	return nil
}

func (l *ConsolidationLock) verifyLockedPID(pid int, priorMtime time.Time) error {
	verified, err := l.readState()
	if err != nil {
		restoreErr := l.restoreUnlocked(priorMtime)
		return errors.Join(err, restoreErr)
	}
	if !verified.exists || !verified.validPID || verified.pid != pid {
		restoreErr := l.restoreUnlocked(priorMtime)
		return errors.Join(
			fmt.Errorf("memory: verify consolidation lock %q: ownership lost to pid %d", l.path, verified.pid),
			restoreErr,
		)
	}
	return nil
}

func (l *ConsolidationLock) restoreUnlocked(priorMtime time.Time) error {
	if priorMtime.IsZero() {
		if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("memory: remove consolidation lock %q during restore: %w", l.path, err)
		}
		return nil
	}

	return l.writeUnlockedAt(priorMtime)
}

func (l *ConsolidationLock) writeUnlockedAt(timestamp time.Time) error {
	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return fmt.Errorf("memory: create lock parent directory for %q: %w", l.path, err)
	}
	if err := os.WriteFile(l.path, nil, lockFilePerm); err != nil {
		return fmt.Errorf("memory: write unlocked consolidation lock %q: %w", l.path, err)
	}
	if err := os.Chtimes(l.path, timestamp, timestamp); err != nil {
		return fmt.Errorf("memory: restore consolidation lock mtime for %q: %w", l.path, err)
	}

	return nil
}

func (l *ConsolidationLock) writeLockedPID(pid int) (returnErr error) {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, lockFilePerm)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("memory: close consolidation lock %q: %w", l.path, err))
		}
	}()
	if _, err := fmt.Fprint(file, strconv.Itoa(pid)); err != nil {
		return fmt.Errorf("memory: write consolidation lock pid: %w", err)
	}
	if err := file.Chmod(lockFilePerm); err != nil {
		return fmt.Errorf("memory: chmod consolidation lock: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("memory: sync consolidation lock: %w", err)
	}
	return nil
}
