package dirpublish

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

// ErrBusy reports another live publisher holding the target's publication lock.
var ErrBusy = errors.New("dirpublish: publication is busy")

// Lock is the cross-process publication lock for one target directory. It is
// an OS lock on a file next to the target, so every process and CompozyOS home
// that publishes the same target serializes on it.
type Lock struct {
	file   *os.File
	locked bool
}

// AcquireLock try-locks the publication lock of targetName inside parent.
// Another holder returns ErrBusy without waiting.
func AcquireLock(parent *fileutil.Directory, targetName string) (_ *Lock, err error) {
	name := LockName(targetName)
	file, err := parent.CreateRegularFile(name, 0o600)
	if errors.Is(err, os.ErrExist) {
		file, err = parent.OpenRegularFile(name)
	}
	if err != nil {
		return nil, fmt.Errorf("dirpublish: open publication lock: %w", err)
	}
	owned := true
	defer func() {
		if !owned {
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("dirpublish: close failed publication lock: %w", closeErr))
		}
	}()

	locked, err := fileutil.TryLockFile(file)
	if err != nil {
		return nil, fmt.Errorf("dirpublish: acquire publication lock: %w", err)
	}
	if !locked {
		return nil, ErrBusy
	}
	owned = false
	return &Lock{file: file, locked: true}, nil
}

// Close releases the lock and closes its file. It is safe to call more than once.
func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	locked := l.locked
	l.file = nil
	l.locked = false

	var unlockErr error
	if locked {
		if err := fileutil.UnlockFile(file); err != nil {
			unlockErr = fmt.Errorf("dirpublish: unlock publication: %w", err)
		}
	}
	var closeErr error
	if err := file.Close(); err != nil {
		closeErr = fmt.Errorf("dirpublish: close publication lock: %w", err)
	}
	return errors.Join(unlockErr, closeErr)
}

// LockName returns the lock file name for targetName, next to the journal.
func LockName(targetName string) string {
	return JournalName(targetName) + ".lock"
}
