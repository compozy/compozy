package registry

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

const installStagingLeaseName = ".compozy-install.lease"

type installStagingLease struct {
	file   *os.File
	locked bool
}

func acquireInstallStagingLease(directory *fileutil.Directory) (_ *installStagingLease, err error) {
	if directory == nil {
		return nil, ErrArchiveRootRequired
	}
	file, err := directory.CreateRegularFile(installStagingLeaseName, 0o600)
	if err != nil {
		return nil, fmt.Errorf("registry: create install staging lease: %w", err)
	}
	owned := true
	defer func() {
		if !owned {
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close failed install staging lease: %w", closeErr))
		}
	}()

	locked, err := tryLockInstallStagingLease(file)
	if err != nil {
		return nil, fmt.Errorf("registry: lock install staging lease: %w", err)
	}
	if !locked {
		return nil, errors.New("registry: newly created install staging lease is already locked")
	}
	owned = false
	return &installStagingLease{file: file, locked: true}, nil
}

func (l *installStagingLease) Close() error {
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
			unlockErr = fmt.Errorf("registry: unlock install staging lease: %w", err)
		}
	}
	var closeErr error
	if err := file.Close(); err != nil {
		closeErr = fmt.Errorf("registry: close install staging lease: %w", err)
	}
	return errors.Join(unlockErr, closeErr)
}

func stagingLeaseIsActive(directory *fileutil.Directory) (_ bool, err error) {
	file, err := directory.OpenRegularFile(installStagingLeaseName)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("registry: open install staging lease: %w", err)
	}
	locked, lockErr := tryLockInstallStagingLease(file)
	if lockErr != nil {
		if closeErr := file.Close(); closeErr != nil {
			lockErr = errors.Join(
				lockErr,
				fmt.Errorf("registry: close install staging lease after lock failure: %w", closeErr),
			)
		}
		return false, fmt.Errorf("registry: inspect install staging lease: %w", lockErr)
	}
	if !locked {
		if closeErr := file.Close(); closeErr != nil {
			return false, fmt.Errorf("registry: close active install staging lease: %w", closeErr)
		}
		return true, nil
	}

	unlockErr := unlockInstallStagingLease(file)
	closeErr := file.Close()
	if unlockErr != nil || closeErr != nil {
		return false, errors.Join(
			wrapOptionalError("registry: unlock inactive install staging lease", unlockErr),
			wrapOptionalError("registry: close inactive install staging lease", closeErr),
		)
	}
	return false, nil
}

func wrapOptionalError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
