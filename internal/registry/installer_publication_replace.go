package registry

import (
	"errors"
	"fmt"
	"os"

	"github.com/compozy/compozy/internal/fileutil"
)

func replaceInstalledDirectory(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	source *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	targetExists, err := installedDirectoryExists(targetParent, targetName)
	if err != nil {
		return nil, fmt.Errorf("registry: inspect install target: %w", err)
	}
	if !targetExists {
		return publishInstalledDirectory(source, targetParent, targetName)
	}

	journal, journalName, err := beginInstallReplacement(targetParent, targetName)
	if err != nil {
		return nil, err
	}
	diagnostics, err := stageInstallReplacementIncoming(targetParent, source, journalName, &journal)
	if err != nil {
		return nil, err
	}
	backupStaged, diagnostics, err := stageInstallReplacementBackup(
		targetParent,
		sourceParent,
		sourceName,
		journalName,
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
		journalName,
		journal,
		backupStaged,
		diagnostics,
	)
}

func beginInstallReplacement(
	targetParent *fileutil.Directory,
	targetName string,
) (installPublicationJournal, string, error) {
	journal, err := newInstallPublicationJournal(targetName)
	if err != nil {
		return installPublicationJournal{}, "", err
	}
	journalName := installPublicationJournalName(targetName)
	if err := writeInstallPublicationJournal(targetParent, journalName, journal, false); err != nil {
		return installPublicationJournal{}, "", err
	}
	return journal, journalName, nil
}

func stageInstallReplacementIncoming(
	targetParent *fileutil.Directory,
	source *fileutil.Directory,
	journalName string,
	journal *installPublicationJournal,
) ([]CleanupDiagnostic, error) {
	diagnostics, err := publishInstalledDirectory(source, targetParent, journal.IncomingName)
	if err != nil {
		cleanupErr := removeInstallPublicationJournal(targetParent, journalName)
		return nil, errors.Join(fmt.Errorf("registry: stage incoming package: %w", err), cleanupErr)
	}
	appendJournalPhaseDiagnostic(targetParent, journalName, journal, installPublicationPhaseIncoming, &diagnostics)
	return diagnostics, nil
}

func stageInstallReplacementBackup(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	journalName string,
	journal *installPublicationJournal,
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
		rollbackErr := rollbackIncomingInstall(targetParent, *journal, sourceParent, sourceName, journalName)
		reportCleanupDiagnostics(diagnostics)
		return false, nil, errors.Join(
			fmt.Errorf("registry: stage installed package backup: %w", backupErr),
			rollbackErr,
		)
	}
	backupStaged := backupErr == nil
	if backupStaged {
		appendJournalPhaseDiagnostic(targetParent, journalName, journal, installPublicationPhaseBackup, &diagnostics)
	}
	return backupStaged, diagnostics, nil
}

func publishInstallReplacementTarget(
	targetParent *fileutil.Directory,
	sourceParent *fileutil.Directory,
	sourceName string,
	journalName string,
	journal installPublicationJournal,
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
			rollbackErr = rollbackInterruptedReplacement(targetParent, journal, sourceParent, sourceName, journalName)
		} else {
			rollbackErr = rollbackIncomingInstall(targetParent, journal, sourceParent, sourceName, journalName)
		}
		reportCleanupDiagnostics(diagnostics)
		return nil, errors.Join(fmt.Errorf("registry: publish updated package: %w", publishErr), rollbackErr)
	}

	appendJournalPhaseDiagnostic(targetParent, journalName, &journal, installPublicationPhasePublished, &diagnostics)
	cleanup, cleanupErr := finalizeCommittedInstallPublication(targetParent, journal, journalName)
	if cleanupErr != nil && len(cleanup) == 0 {
		cleanup, _ = appendCleanupDiagnostic(cleanup, CleanupOperationPublication)
	}
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, cleanup)
	return diagnostics, nil
}
