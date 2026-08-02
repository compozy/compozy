package registry

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

type installStaging struct {
	parent        *fileutil.Directory
	temporary     *fileutil.Directory
	lease         *installStagingLease
	temporaryName string
	targetName    string
	targetPath    string
}

func (i *Installer) newInstallStaging(
	absTarget string,
) (_ *installStaging, diagnostics []CleanupDiagnostic, err error) {
	parent, targetName, err := fileutil.OpenParentDirectory(absTarget)
	if err != nil {
		return nil, nil, fmt.Errorf("registry: open install parent: %w", err)
	}
	parentOwned := true
	defer func() {
		if !parentOwned {
			return
		}
		if closeErr := parent.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close install parent: %w", closeErr))
		}
	}()

	staleDiagnostics, err := i.cleanupStaleTempDirs(parent)
	if err != nil {
		return nil, nil, err
	}
	temporaryName, temporary, err := parent.CreateTemporaryDirectory(".compozy-install-", 0o700)
	if err != nil {
		return nil, nil, fmt.Errorf("registry: create temporary install directory: %w", err)
	}
	lease, err := acquireInstallStagingLease(temporary)
	if err != nil {
		closeErr := temporary.Close()
		removeErr := parent.RemoveAll(temporaryName)
		return nil, nil, errors.Join(
			err,
			wrapOptionalError("registry: close temporary install directory after lease failure", closeErr),
			wrapOptionalError("registry: remove temporary install directory after lease failure", removeErr),
		)
	}
	parentOwned = false
	return &installStaging{
		parent: parent, temporary: temporary, lease: lease, temporaryName: temporaryName,
		targetName: targetName, targetPath: absTarget,
	}, staleDiagnostics, nil
}

func (i *Installer) cleanupInstallStaging(staging *installStaging) (CleanupOperation, error) {
	if staging == nil {
		return "", nil
	}
	var cleanupErr error
	if staging.lease != nil {
		if closeErr := staging.lease.Close(); closeErr != nil {
			cleanupErr = errors.Join(cleanupErr, closeErr)
		}
		staging.lease = nil
	}
	if staging.temporary != nil {
		if closeErr := staging.temporary.Close(); closeErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("close temporary install directory: %w", closeErr))
		}
		staging.temporary = nil
	}
	if staging.parent != nil {
		removeErr := i.removeTemporaryDirectory(staging.parent, staging.temporaryName)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove temporary install directory: %w", removeErr))
		}
		if closeErr := staging.parent.Close(); closeErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("close install parent: %w", closeErr))
		}
		staging.parent = nil
	}
	if cleanupErr == nil {
		return "", nil
	}
	return CleanupOperationStagingTemp, cleanupErr
}

func (i *Installer) cleanupStaleTempDirs(parent *fileutil.Directory) ([]CleanupDiagnostic, error) {
	names, err := parent.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("registry: read install parent: %w", err)
	}
	diagnostics := make([]CleanupDiagnostic, 0, 1)
	cutoff := i.now().Add(-i.tempDirMaxAge)
	for _, name := range names {
		if !strings.HasPrefix(name, ".compozy-install-") {
			continue
		}
		directory, openErr := parent.OpenDirectory(name)
		if errors.Is(openErr, fileutil.ErrNotDirectory) {
			continue
		}
		if openErr != nil {
			var appended bool
			diagnostics, appended = appendCleanupDiagnostic(diagnostics, CleanupOperationStaleTemp)
			if appended {
				reportInstallCleanup(CleanupOperationStaleTemp)
			}
			continue
		}
		active, leaseErr := stagingLeaseIsActive(directory)
		info, statErr := directory.Stat()
		closeErr := directory.Close()
		if leaseErr != nil || statErr != nil || closeErr != nil {
			var appended bool
			diagnostics, appended = appendCleanupDiagnostic(diagnostics, CleanupOperationStaleTemp)
			if appended {
				reportInstallCleanup(CleanupOperationStaleTemp)
			}
			continue
		}
		if active {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		removeErr := i.removeTemporaryDirectory(parent, name)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			var appended bool
			diagnostics, appended = appendCleanupDiagnostic(diagnostics, CleanupOperationStaleTemp)
			if appended {
				reportInstallCleanup(CleanupOperationStaleTemp)
			}
		}
	}
	return diagnostics, nil
}
