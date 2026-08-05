package registry

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

type installPublicationLock struct {
	file   *os.File
	locked bool
}

func acquireInstallPublicationLock(
	parent *fileutil.Directory,
	targetName string,
) (_ *installPublicationLock, err error) {
	name := installPublicationLockName(targetName)
	file, err := parent.CreateRegularFile(name, 0o600)
	if errors.Is(err, os.ErrExist) {
		file, err = parent.OpenRegularFile(name)
	}
	if err != nil {
		return nil, fmt.Errorf("registry: open install publication lock: %w", err)
	}
	owned := true
	defer func() {
		if !owned {
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close failed install publication lock: %w", closeErr))
		}
	}()

	locked, err := tryLockInstallStagingLease(file)
	if err != nil {
		return nil, fmt.Errorf("registry: acquire install publication lock: %w", err)
	}
	if !locked {
		return nil, ErrInstallPublicationBusy
	}
	owned = false
	return &installPublicationLock{file: file, locked: true}, nil
}

func (l *installPublicationLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	locked := l.locked
	l.file = nil
	l.locked = false

	var unlockErr error
	if locked {
		if err := unlockInstallStagingLease(file); err != nil {
			unlockErr = fmt.Errorf("registry: unlock install publication: %w", err)
		}
	}
	var closeErr error
	if err := file.Close(); err != nil {
		closeErr = fmt.Errorf("registry: close install publication lock: %w", err)
	}
	return errors.Join(unlockErr, closeErr)
}

func installPublicationLockName(targetName string) string {
	return installPublicationJournalName(targetName) + ".lock"
}
