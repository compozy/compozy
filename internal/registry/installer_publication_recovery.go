package registry

import (
	"errors"

	"github.com/compozy/compozy/internal/dirpublish"
	"github.com/compozy/compozy/internal/fileutil"
)

// rollForwardPolicy is the registry's dirpublish recovery policy: an
// interrupted package publication completes toward the incoming package
// whenever the layout proves which directory is which, and stops for operator
// inspection otherwise. It collects the cleanup diagnostics of the recovery.
type rollForwardPolicy struct {
	diagnostics []CleanupDiagnostic
}

func recoverInstalledDirPublication(
	parent *fileutil.Directory,
	targetName string,
) ([]CleanupDiagnostic, error) {
	policy := &rollForwardPolicy{diagnostics: make([]CleanupDiagnostic, 0, 1)}
	if err := dirpublish.Recover(parent, targetName, policy); err != nil {
		return nil, err
	}
	return policy.diagnostics, nil
}

func (p *rollForwardPolicy) RecoverLayout(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
	layout dirpublish.Layout,
) (err error) {
	var diagnostics []CleanupDiagnostic
	switch layout {
	case dirpublish.HasTarget | dirpublish.HasIncoming | dirpublish.HasBackup:
		return dirpublish.RecoveryError("resolve three competing publication directories", nil)
	case dirpublish.HasTarget | dirpublish.HasBackup:
		if journal.Phase != dirpublish.PhasePublished {
			return dirpublish.RecoveryError("verify committed target before removing backup", nil)
		}
		diagnostics, err = finalizeRecoveredInstallPublication(parent, journal, p.diagnostics, nil)
	case dirpublish.HasTarget | dirpublish.HasIncoming:
		diagnostics, err = recoverInstallTargetAndIncoming(parent, journal, p.diagnostics)
	case dirpublish.HasIncoming | dirpublish.HasBackup:
		diagnostics, err = recoverInstallIncomingWithBackup(parent, journal, p.diagnostics)
	case dirpublish.HasBackup:
		diagnostics, err = recoverInstallBackup(parent, journal, p.diagnostics)
	case dirpublish.HasIncoming:
		diagnostics, err = recoverInstallIncoming(parent, journal, p.diagnostics)
	case dirpublish.HasTarget:
		diagnostics, err = recoverSettledInstallTarget(parent, journal, p.diagnostics)
	default:
		return dirpublish.RecoveryError("resolve incomplete publication state", nil)
	}
	if err != nil {
		return err
	}
	p.diagnostics = diagnostics
	return nil
}

func recoverInstallTargetAndIncoming(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
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
		return nil, errors.Join(dirpublish.RecoveryError("stage canonical target during recovery", err), cleanupErr)
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
		return nil, errors.Join(dirpublish.RecoveryError("publish incoming target during recovery", err), cleanupErr)
	}
	return finalizeRecoveredInstallPublication(parent, journal, diagnostics, cleanupErr)
}

func recoverInstallIncomingWithBackup(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
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
		return nil, errors.Join(dirpublish.RecoveryError("publish staged incoming target", err), cleanupErr)
	}
	return finalizeRecoveredInstallPublication(parent, journal, diagnostics, cleanupErr)
}

func recoverInstallBackup(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
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
		return nil, errors.Join(dirpublish.RecoveryError("restore staged backup", err), cleanupErr)
	}
	if err := dirpublish.RemoveJournal(parent, journal.TargetName); err != nil {
		return nil, errors.Join(dirpublish.RecoveryError("remove restored publication journal", err), cleanupErr)
	}
	return diagnostics, nil
}

func recoverInstallIncoming(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
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
		return nil, errors.Join(dirpublish.RecoveryError("publish orphaned incoming target", err), cleanupErr)
	}
	if err := dirpublish.RemoveJournal(parent, journal.TargetName); err != nil {
		return nil, errors.Join(dirpublish.RecoveryError("remove recovered publication journal", err), cleanupErr)
	}
	return diagnostics, nil
}

func recoverSettledInstallTarget(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
	diagnostics []CleanupDiagnostic,
) ([]CleanupDiagnostic, error) {
	// Only the target is left: nothing remains to roll forward or back in any phase (a rollback that
	// restored the target, or a finalize that removed the backup), so only the journal goes.
	if err := dirpublish.RemoveJournal(parent, journal.TargetName); err != nil {
		return nil, dirpublish.RecoveryError("remove settled publication journal", err)
	}
	return diagnostics, nil
}

func finalizeRecoveredInstallPublication(
	parent *fileutil.Directory,
	journal dirpublish.Journal,
	diagnostics []CleanupDiagnostic,
	priorCleanupErr error,
) ([]CleanupDiagnostic, error) {
	cleanup, cleanupErr := finalizeCommittedInstallPublication(parent, journal)
	cleanupErr = errors.Join(priorCleanupErr, cleanupErr)
	diagnostics = appendUniqueCleanupDiagnostics(diagnostics, cleanup)
	if len(cleanup) > 0 {
		return nil, errors.Join(dirpublish.RecoveryError("clean recovered publication state", nil), cleanupErr)
	}
	return diagnostics, nil
}
