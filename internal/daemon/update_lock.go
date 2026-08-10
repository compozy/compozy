package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/gofrs/flock"
)

// ErrRuntimeMutationInProgress reports that another process owns update.lock.
var ErrRuntimeMutationInProgress = errors.New("daemon: runtime mutation in progress")

// UpdateLockOwner is the durable process identity stored inside update.lock.
type UpdateLockOwner struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// UpdateLock owns the cross-process runtime mutation lock.
type UpdateLock struct {
	flock      *flock.Flock
	path       string
	staleOwner *UpdateLockOwner
}

// AcquireUpdateLock locks update.lock and records the caller's process identity.
func AcquireUpdateLock(path string, owner UpdateLockOwner) (*UpdateLock, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("daemon: update lock path is required")
	}
	if owner.PID <= 0 || owner.StartedAt.IsZero() {
		return nil, errors.New("daemon: update lock owner is invalid")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("daemon: create update lock directory for %q: %w", cleanPath, err)
	}

	fileLock := flock.New(cleanPath, flock.SetPermissions(0o600))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, fmt.Errorf("daemon: acquire update lock %q: %w", cleanPath, err)
	}
	if !locked {
		if closeErr := fileLock.Close(); closeErr != nil {
			return nil, errors.Join(ErrRuntimeMutationInProgress, closeErr)
		}
		return nil, ErrRuntimeMutationInProgress
	}
	priorOwner, err := readUpdateLockOwner(cleanPath)
	if err != nil {
		unlockErr := fileLock.Unlock()
		closeErr := fileLock.Close()
		return nil, errors.Join(err, unlockErr, closeErr)
	}
	var staleOwner *UpdateLockOwner
	if priorOwner != nil && !procutil.MatchesStartTime(priorOwner.PID, priorOwner.StartedAt) {
		staleOwner = priorOwner
	}
	if err := writeUpdateLockOwner(cleanPath, owner); err != nil {
		unlockErr := fileLock.Unlock()
		closeErr := fileLock.Close()
		return nil, errors.Join(err, unlockErr, closeErr)
	}
	return &UpdateLock{flock: fileLock, path: cleanPath, staleOwner: staleOwner}, nil
}

// StaleOwner reports the stale identity replaced during acquisition, if any.
func (l *UpdateLock) StaleOwner() *UpdateLockOwner {
	if l == nil || l.staleOwner == nil {
		return nil
	}
	owner := *l.staleOwner
	return &owner
}

// Release clears the owner payload, unlocks, and closes update.lock.
func (l *UpdateLock) Release() error {
	if l == nil || l.flock == nil {
		return nil
	}
	var errs []error
	if err := clearUpdateLockOwner(l.path); err != nil {
		errs = append(errs, err)
	}
	if err := l.flock.Unlock(); err != nil {
		errs = append(errs, fmt.Errorf("daemon: unlock update lock %q: %w", l.path, err))
	}
	if err := l.flock.Close(); err != nil {
		errs = append(errs, fmt.Errorf("daemon: close update lock %q: %w", l.path, err))
	}
	l.flock = nil
	l.staleOwner = nil
	return errors.Join(errs...)
}

func readUpdateLockOwner(path string) (*UpdateLockOwner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("daemon: read update lock %q: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}
	var owner UpdateLockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return nil, nil
	}
	if owner.PID <= 0 || owner.StartedAt.IsZero() {
		return nil, nil
	}
	return &owner, nil
}

func writeUpdateLockOwner(path string, owner UpdateLockOwner) (returnErr error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("daemon: open update lock %q: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("daemon: close update lock %q: %w", path, err))
		}
	}()
	if err := json.NewEncoder(file).Encode(owner); err != nil {
		return fmt.Errorf("daemon: encode update lock %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("daemon: sync update lock %q: %w", path, err)
	}
	return nil
}

func clearUpdateLockOwner(path string) (returnErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("daemon: clear update lock %q: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("daemon: close cleared update lock %q: %w", path, err))
		}
	}()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("daemon: sync cleared update lock %q: %w", path, err)
	}
	return nil
}
