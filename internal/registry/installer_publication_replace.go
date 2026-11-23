package registry

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
)

func replaceInstalledDirectory(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	source *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	targetExists, err := dirpublish.DirectoryExists(targetParent, targetName)
	if err != nil {
		return nil, fmt.Errorf("registry: inspect install target: %w", err)
	}
	if !targetExists {
		return publishInstalledDirectory(source, targetParent, targetName)
	}

	journal, err := beginInstallReplacement(targetParent, targetName)
	if err != nil {
		return nil, err
	}
	diagnostics, err := stageInstallReplacementIncoming(targetParent, source, &journal)
	if err != nil {
		return nil, err
	}
	backupStaged, diagnostics, err := stageInstallReplacementBackup(
		targetParent,
		sourceParent,
		sourceName,
		&journal,
		diagnostics,
	)
	if err != nil {
		return nil, err
	}
	return publishInstallReplacementTarget(
		targetParent,
		sourceParent,
		sourceName,
		journal,
		backupStaged,
		diagnostics,
	)
}

func beginInstallReplacement(
	targetParent *fileutil.Directory,
	targetName string,
) (dirpublish.Journal, error) {
	journal, err := dirpublish.NewJournal(targetName, nil)
	if err != nil {
		return dirpublish.Journal{}, err
	}
	if err := dirpublish.WriteJournal(targetParent, journal, false); err != nil {
		return dirpublish.Journal{}, err
	}
	return journal, nil
}

func stageInstallReplacementIncoming(
	targetParent *fileutil.Directory,
	source *fileutil.Directory,
	journal *dirpublish.Journal,
) ([]CleanupDiagnostic, error) {
	diagnostics, err := publishInstalledDirectory(source, targetParent, journal.IncomingName)
	if err != nil {
		cleanupErr := dirpublish.RemoveJournal(targetParent, journal.TargetName)
		return nil, errors.Join(fmt.Errorf("registry: stage incoming package: %w", err), cleanupErr)
	}
	appendJournalPhaseDiagnostic(targetParent, journal, dirpublish.PhaseIncoming, &diagnostics)
	return diagnostics, nil
}

func stageInstallReplacementBackup(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	journal *dirpublish.Journal,
	diagnostics []CleanupDiagnostic,
) (bool, []CleanupDiagnostic, error) {
	backupDiagnostics, backupErr := moveNamedInstalledDirectory(
		targetParent,
		journal.TargetName,
		targetParent,
		journal.BackupName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, backupDiagnostics)
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		rollbackErr := rollbackIncomingInstall(targetParent, *journal, sourceParent, sourceName)
		reportCleanupDiagnostics(diagnostics)
		return false, nil, errors.Join(
			fmt.Errorf("registry: stage installed package backup: %w", backupErr),
			rollbackErr,
		)
	}
	backupStaged := backupErr == nil
	if backupStaged {
		appendJournalPhaseDiagnostic(targetParent, journal, dirpublish.PhaseBackup, &diagnostics)
	}
	return backupStaged, diagnostics, nil
}

func publishInstallReplacementTarget(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	journal dirpublish.Journal,
	backupStaged bool,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	publishDiagnostics, publishErr := moveNamedInstalledDirectory(
		targetParent,
		journal.IncomingName,
		targetParent,
		journal.TargetName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, publishDiagnostics)
	if publishErr != nil {
		var rollbackErr error
		if backupStaged {
			rollbackErr = rollbackInterruptedReplacement(targetParent, journal, sourceParent, sourceName)
		} else {
			rollbackErr = rollbackIncomingInstall(targetParent, journal, sourceParent, sourceName)
		}
		reportCleanupDiagnostics(diagnostics)
		return nil, errors.Join(fmt.Errorf("registry: publish updated package: %w", publishErr), rollbackErr)
	}

	appendJournalPhaseDiagnostic(targetParent, &journal, dirpublish.PhasePublished, &diagnostics)
	cleanup, cleanupErr := finalizeCommittedInstallPublication(targetParent, journal)
	if cleanupErr != nil && len(cleanup) == 0 {
		cleanup, _ = appendCleanupDiagnostic(cleanup, CleanupOperationPublication)
	}
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, cleanup)
	return diagnostics, nil
}

func rollbackIncomingInstall(
	targetParent *fileutil.Directory,
	journal dirpublish.Journal,
	sourceParent *fileutil.Directory,
	sourceName string,
) error {
	_, restoreErr := moveNamedInstalledDirectory(
		targetParent,
		journal.IncomingName,
		sourceParent,
		sourceName,
	)
	if restoreErr != nil {
		return dirpublish.RecoveryError("restore incoming source", restoreErr)
	}
	if err := dirpublish.RemoveJournal(targetParent, journal.TargetName); err != nil {
		return dirpublish.RecoveryError("remove rolled-back publication journal", err)
	}
	return nil
}

func rollbackInterruptedReplacement(
	targetParent *fileutil.Directory,
	journal dirpublish.Journal,
	sourceParent *fileutil.Directory,
	sourceName string,
) error {
	_, restoreTargetErr := moveNamedInstalledDirectory(
		targetParent,
		journal.BackupName,
		targetParent,
		journal.TargetName,
	)
	_, restoreSourceErr := moveNamedInstalledDirectory(
		targetParent,
		journal.IncomingName,
		sourceParent,
		sourceName,
	)
	if restoreTargetErr != nil || restoreSourceErr != nil {
		return errors.Join(
			dirpublish.RecoveryStepError("restore canonical target", restoreTargetErr),
			dirpublish.RecoveryStepError("restore incoming source", restoreSourceErr),
		)
	}
	if err := dirpublish.RemoveJournal(targetParent, journal.TargetName); err != nil {
		return dirpublish.RecoveryError("remove rolled-back publication journal", err)
	}
	return nil
}

// appendJournalPhaseDiagnostic advances the journal and records a failed phase
// write as a publication cleanup diagnostic: the registry's roll-forward
// recovery completes the publication from any later phase.
func appendJournalPhaseDiagnostic(
	parent *fileutil.Directory,
	journal *dirpublish.Journal,
	phase string,
	diagnostics *[]CleanupDiagnostic,
) {
	if err := dirpublish.AdvancePhase(parent, journal, phase); err != nil {
		*diagnostics, _ = appendCleanupDiagnostic(*diagnostics, CleanupOperationPublication)
	}
}

// finalizeCommittedInstallPublication removes the backup and then the journal of a published
// replacement. Both survive until the published phase is durable, so a failed phase write or a
// failed removal always leaves a layout recovery finishes rather than one it must refuse.
func finalizeCommittedInstallPublication(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
) ([]CleanupDiagnostic, error) {
	diagnostics := make([]CleanupDiagnostic, 0, 1)
	if err := ensureDurablePublished(parent, &journal); err != nil {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationPublication)
		return diagnostics, recordInstallCleanupFailure(CleanupOperationPublication, err)
	}
	if err := parent.RemoveAll(journal.BackupName); err != nil && !errors.Is(err, os.ErrNotExist) {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationBackup)
		return diagnostics, recordInstallCleanupFailure(CleanupOperationBackup, err)
	}
	if err := dirpublish.RemoveJournal(parent, journal.TargetName); err != nil {
		diagnostics, _ = appendCleanupDiagnostic(diagnostics, CleanupOperationPublication)
		return diagnostics, recordInstallCleanupFailure(CleanupOperationPublication, err)
	}
	return diagnostics, nil
}

// ensureDurablePublished records the published phase before anything committed is removed. A
// write that landed without its directory sync counts once the directory sync succeeds.
func ensureDurablePublished(parent *fileutil.Directory, journal *dirpublish.Journal) error {
	if journal.Phase == dirpublish.PhasePublished {
		return nil
	}
	err := dirpublish.AdvancePhase(parent, journal, dirpublish.PhasePublished)
	if err == nil || !errors.Is(err, fileutil.ErrPublishedNotDurable) {
		return err
	}
	if syncErr := parent.Sync(); syncErr != nil {
		return errors.Join(err, syncErr)
	}
	journal.Phase = dirpublish.PhasePublished
	return nil
}

func reportCleanupDiagnostics(diagnostics []CleanupDiagnostic) {
	for _, diagnostic := range diagnostics {
		reportInstallCleanup(diagnostic.Operation)
	}
}
