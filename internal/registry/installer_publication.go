package registry

import (
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/fileutil"
)

// ErrInstallTargetExists reports an atomic no-replace publication collision.
var ErrInstallTargetExists = errors.New("registry: install target already exists")

// ErrInstallPublicationRecoveryRequired reports an interrupted replacement
// whose filesystem state cannot be resolved without operator inspection.
var ErrInstallPublicationRecoveryRequired = errors.New("registry: install publication recovery required")

// ErrInstallPublicationBusy reports another live publisher for the same target.
var ErrInstallPublicationBusy = errors.New("registry: install publication is busy")

// MoveInstalledDir moves an extracted package directory into its final location.
// It never creates the target parent. A committed publication remains success
// even when a subsequent durability or cleanup step reports a diagnostic.
func MoveInstalledDir(
	extractedDir string,
	targetDir string,
	replaceExisting bool,
) (result *MoveInstalledDirResult, err error) {
	sourceParent, sourceName, err := fileutil.OpenParentDirectory(extractedDir)
	if err != nil {
		return nil, fmt.Errorf("registry: open extracted package parent: %w", err)
	}
	defer func() {
		closeErr := sourceParent.Close()
		if closeErr == nil {
			return
		}
		if result != nil && result.Committed && err == nil {
			result.CleanupDiagnostics, _ = appendCleanupDiagnostic(
				result.CleanupDiagnostics,
				CleanupOperationSourceParent,
			)
			reportInstallCleanup(CleanupOperationSourceParent)
			return
		}
		err = errors.Join(err, fmt.Errorf("registry: close extracted package parent: %w", closeErr))
	}()
	targetParent, targetName, err := fileutil.OpenParentDirectory(targetDir)
	if err != nil {
		return nil, fmt.Errorf("registry: open install target parent: %w", err)
	}
	defer func() {
		closeErr := targetParent.Close()
		if closeErr == nil {
			return
		}
		if result != nil && result.Committed && err == nil {
			result.CleanupDiagnostics, _ = appendCleanupDiagnostic(
				result.CleanupDiagnostics,
				CleanupOperationTargetParent,
			)
			reportInstallCleanup(CleanupOperationTargetParent)
			return
		}
		err = errors.Join(err, fmt.Errorf("registry: close install target parent: %w", closeErr))
	}()

	source, err := sourceParent.OpenDirectoryForMove(sourceName)
	if err != nil {
		return nil, fmt.Errorf("registry: open extracted package for publication: %w", err)
	}
	defer func() {
		closeErr := source.Close()
		if closeErr == nil {
			return
		}
		if result != nil && result.Committed && err == nil {
			result.CleanupDiagnostics, _ = appendCleanupDiagnostic(
				result.CleanupDiagnostics,
				CleanupOperationSourceParent,
			)
			reportInstallCleanup(CleanupOperationSourceParent)
			return
		}
		err = errors.Join(err, fmt.Errorf("registry: close extracted package handle: %w", closeErr))
	}()

	diagnostics, err := moveInstalledPackage(
		targetParent,
		sourceParent,
		sourceName,
		source,
		targetName,
		replaceExisting,
	)
	if err != nil {
		return nil, err
	}
	result = &MoveInstalledDirResult{Committed: true, CleanupDiagnostics: diagnostics}
	for _, diagnostic := range diagnostics {
		reportInstallCleanup(diagnostic.Operation)
	}
	return result, nil
}

func moveInstalledPackage(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	source *fileutil.Directory,
	targetName string,
	replaceExisting bool,
) (diagnostics []CleanupDiagnostic, err error) {
	publicationLock, err := acquireInstallPublicationLock(targetParent, targetName)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := publicationLock.Close()
		if closeErr == nil {
			return
		}
		if err == nil {
			diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationPublication)
			return
		}
		err = joinCleanupFailure(err, CleanupOperationPublication, closeErr)
	}()

	diagnostics, err = recoverInstalledDirPublication(targetParent, targetName)
	if err != nil {
		return nil, err
	}
	if !replaceExisting {
		publishedDiagnostics, publishErr := publishInstalledDirectory(source, targetParent, targetName)
		return appendUniqueCleanupDiagnostics(diagnostics, publishedDiagnostics), publishErr
	}
	replacementDiagnostics, replaceErr := replaceInstalledDirectory(
		targetParent,
		sourceParent,
		sourceName,
		source,
		targetName,
	)
	return appendUniqueCleanupDiagnostics(diagnostics, replacementDiagnostics), replaceErr
}

func publishInstalledDirectory(
	source *fileutil.Directory,
	targetParent *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	result, err := source.MoveTo(targetParent, targetName, false)
	if err != nil {
		return nil, normalizePublicationError(err)
	}
	if !result.Committed() {
		return nil, errors.New("registry: install publication returned without a committed target")
	}
	if result.PostCommitErr == nil {
		return nil, nil
	}
	return []CleanupDiagnostic{{Operation: CleanupOperationPublication}}, nil
}

func moveNamedInstalledDirectory(
	sourceParent *fileutil.Directory,
	sourceName string,
	targetParent *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	diagnostics, cleanupErr, err := moveNamedInstalledDirectoryEvidence(
		sourceParent,
		sourceName,
		targetParent,
		targetName,
	)
	if err != nil {
		return nil, errors.Join(err, cleanupErr)
	}
	if cleanupErr != nil && len(diagnostics) == 0 {
		return nil, errors.Join(
			errors.New("registry: directory move cleanup failed without a diagnostic"),
			cleanupErr,
		)
	}
	return diagnostics, nil
}

func moveNamedInstalledDirectoryEvidence(
	sourceParent *fileutil.Directory,
	sourceName string,
	targetParent *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error, error) {
	source, err := sourceParent.OpenDirectoryForMove(sourceName)
	if err != nil {
		return nil, nil, err
	}
	result, moveErr := source.MoveTo(targetParent, targetName, false)
	closeErr := source.Close()
	if moveErr != nil {
		return nil, nil, errors.Join(moveErr, closeErr)
	}
	if !result.Committed() {
		return nil, nil, errors.Join(
			errors.New("registry: directory move returned without a committed target"),
			closeErr,
		)
	}
	diagnostics := make([]CleanupDiagnostic, 0, 1)
	cleanupCause := errors.Join(result.PostCommitErr, closeErr)
	if cleanupCause != nil {
		diagnostics = append(diagnostics, CleanupDiagnostic{Operation: CleanupOperationPublication})
	}
	return diagnostics, recordInstallCleanupFailure(CleanupOperationPublication, cleanupCause), nil
}

func appendUniqueCleanupDiagnostics(
	diagnostics []CleanupDiagnostic,
	additional []CleanupDiagnostic,
) []CleanupDiagnostic {
	for _, diagnostic := range additional {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, diagnostic.Operation)
	}
	return diagnostics
}

func normalizePublicationError(err error) error {
	if errors.Is(err, fileutil.ErrTargetExists) {
		return ErrInstallTargetExists
	}
	return err
}
