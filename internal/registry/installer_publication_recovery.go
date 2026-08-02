package registry

import (
	"errors"

	"github.com/compozy/compozy/internal/fileutil"
)

type installPublicationLayout uint8

const (
	installPublicationHasTarget installPublicationLayout = 1 << iota
	installPublicationHasIncoming
	installPublicationHasBackup
)

func recoverInstalledDirPublication(
	parent *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	journalName := installPublicationJournalName(targetName)
	journal, exists, err := readInstallPublicationJournal(parent, journalName, targetName)
	if err != nil || !exists {
		return nil, err
	}
	layout, err := inspectInstallPublicationLayout(parent, journal)
	if err != nil {
		return nil, err
	}
	diagnostics := make([]CleanupDiagnostic, 0, 1)
	switch layout {
	case installPublicationHasTarget | installPublicationHasIncoming | installPublicationHasBackup:
		return nil, publicationRecoveryError("resolve three competing publication directories", nil)
	case installPublicationHasTarget | installPublicationHasBackup:
		if journal.Phase != installPublicationPhasePublished {
			return nil, publicationRecoveryError("verify committed target before removing backup", nil)
		}
		return finalizeRecoveredInstallPublication(parent, journal, journalName, diagnostics, nil)
	case installPublicationHasTarget | installPublicationHasIncoming:
		return recoverInstallTargetAndIncoming(parent, journal, journalName, diagnostics)
	case installPublicationHasIncoming | installPublicationHasBackup:
		return recoverInstallIncomingWithBackup(parent, journal, journalName, diagnostics)
	case installPublicationHasBackup:
		return recoverInstallBackup(parent, journal, journalName, diagnostics)
	case installPublicationHasIncoming:
		return recoverInstallIncoming(parent, journal, journalName, diagnostics)
	case installPublicationHasTarget:
		return recoverSettledInstallTarget(parent, journal, journalName, diagnostics)
	default:
		return nil, publicationRecoveryError("resolve incomplete publication state", nil)
	}
}

func inspectInstallPublicationLayout(
	parent *fileutil.Directory,
	journal installPublicationJournal,
) (installPublicationLayout, error) {
	checks := []struct {
		name      string
		operation string
		flag      installPublicationLayout
	}{
		{name: journal.TargetName, operation: "inspect canonical target", flag: installPublicationHasTarget},
		{name: journal.IncomingName, operation: "inspect incoming target", flag: installPublicationHasIncoming},
		{name: journal.BackupName, operation: "inspect backup target", flag: installPublicationHasBackup},
	}
	var layout installPublicationLayout
	for _, check := range checks {
		exists, err := installedDirectoryExists(parent, check.name)
		if err != nil {
			return 0, publicationRecoveryError(check.operation, err)
		}
		if exists {
			layout |= check.flag
		}
	}
	return layout, nil
}

func recoverInstallTargetAndIncoming(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	moved, cleanupErr, err := moveNamedInstalledDirectoryEvidence(
		parent,
		journal.TargetName,
		parent,
		journal.BackupName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, moved)
	if err != nil {
		return nil, errors.Join(publicationRecoveryError("stage canonical target during recovery", err), cleanupErr)
	}
	moved, nextCleanupErr, err := moveNamedInstalledDirectoryEvidence(
		parent,
		journal.IncomingName,
		parent,
		journal.TargetName,
	)
	cleanupErr = errors.Join(cleanupErr, nextCleanupErr)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, moved)
	if err != nil {
		return nil, errors.Join(publicationRecoveryError("publish incoming target during recovery", err), cleanupErr)
	}
	return finalizeRecoveredInstallPublication(parent, journal, journalName, diagnostics, cleanupErr)
}

func recoverInstallIncomingWithBackup(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	moved, cleanupErr, err := moveNamedInstalledDirectoryEvidence(
		parent,
		journal.IncomingName,
		parent,
		journal.TargetName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, moved)
	if err != nil {
		return nil, errors.Join(publicationRecoveryError("publish staged incoming target", err), cleanupErr)
	}
	return finalizeRecoveredInstallPublication(parent, journal, journalName, diagnostics, cleanupErr)
}

func recoverInstallBackup(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	moved, cleanupErr, err := moveNamedInstalledDirectoryEvidence(
		parent,
		journal.BackupName,
		parent,
		journal.TargetName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, moved)
	if err != nil {
		return nil, errors.Join(publicationRecoveryError("restore staged backup", err), cleanupErr)
	}
	if err := removeInstallPublicationJournal(parent, journalName); err != nil {
		return nil, errors.Join(publicationRecoveryError("remove restored publication journal", err), cleanupErr)
	}
	return diagnostics, nil
}

func recoverInstallIncoming(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	moved, cleanupErr, err := moveNamedInstalledDirectoryEvidence(
		parent,
		journal.IncomingName,
		parent,
		journal.TargetName,
	)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, moved)
	if err != nil {
		return nil, errors.Join(publicationRecoveryError("publish orphaned incoming target", err), cleanupErr)
	}
	if err := removeInstallPublicationJournal(parent, journalName); err != nil {
		return nil, errors.Join(publicationRecoveryError("remove recovered publication journal", err), cleanupErr)
	}
	return diagnostics, nil
}

func recoverSettledInstallTarget(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	if journal.Phase != installPublicationPhasePrepared && journal.Phase != installPublicationPhasePublished {
		return nil, publicationRecoveryError("resolve incomplete publication state", nil)
	}
	if err := removeInstallPublicationJournal(parent, journalName); err != nil {
		return nil, publicationRecoveryError("remove settled publication journal", err)
	}
	return diagnostics, nil
}

func finalizeRecoveredInstallPublication(
	parent *fileutil.Directory,
	journal installPublicationJournal,
	journalName string,
	diagnostics []CleanupDiagnostic,
	priorCleanupErr error,
) ([]CleanupDiagnostic, error) {
	cleanup, cleanupErr := finalizeCommittedInstallPublication(parent, journal, journalName)
	cleanupErr = errors.Join(priorCleanupErr, cleanupErr)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, cleanup)
	if len(cleanup) > 0 {
		return nil, errors.Join(publicationRecoveryError("clean recovered publication state", nil), cleanupErr)
	}
	return diagnostics, nil
}
