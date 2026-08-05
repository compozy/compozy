package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

func backupSessionDBManifestArtifacts(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
) error {
	if err := manifest.requireOwner(owner); err != nil {
		return err
	}
	if err := requireSessionDBClearBackupState(dbPath, manifest, nil); err != nil {
		return err
	}
	moved := make([]sessionDBClearArtifact, 0, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if err := requireSessionDBClearBackupState(dbPath, manifest, moved); err != nil {
			return rollbackSessionDBManifestBackup(ctx, lease, owner, dbPath, moved, err)
		}
		original := dbPath + artifact.Suffix
		bound, err := verifySessionDBClearArtifact(ctx, lease, owner, original, artifact)
		if err != nil {
			return rollbackSessionDBManifestBackup(
				ctx,
				lease,
				owner,
				dbPath,
				moved,
				err,
			)
		}
		committed, moveErr := bound.MoveTo(original + ".clear-backup")
		closeErr := bound.Close()
		if committed {
			moved = append(moved, artifact)
		}
		if moveErr != nil || closeErr != nil {
			return rollbackSessionDBManifestBackup(
				ctx,
				lease,
				owner,
				dbPath,
				moved,
				errors.Join(moveErr, closeErr),
			)
		}
		if err := requireSessionDBClearBackupState(dbPath, manifest, moved); err != nil {
			return rollbackSessionDBManifestBackup(ctx, lease, owner, dbPath, moved, err)
		}
	}
	return nil
}

func rollbackSessionDBManifestBackup(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	moved []sessionDBClearArtifact,
	primaryErr error,
) error {
	var rollbackErr error
	for _, artifact := range slices.Backward(moved) {
		backup := dbPath + artifact.Suffix + ".clear-backup"
		bound, err := verifySessionDBClearArtifact(ctx, lease, owner, backup, artifact)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
			continue
		}
		_, moveErr := bound.MoveTo(dbPath + artifact.Suffix)
		rollbackErr = errors.Join(rollbackErr, moveErr, bound.Close())
	}
	if rollbackErr == nil {
		rollbackErr = discardSessionDBClearManifest(dbPath)
	}
	return errors.Join(primaryErr, rollbackErr)
}

func restoreSessionDBManifestArtifacts(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
) error {
	if err := manifest.requireOwner(owner); err != nil {
		return err
	}
	if err := requireNoUnexpectedSessionDBClearBackups(dbPath, manifest); err != nil {
		return err
	}
	if err := preflightSessionDBManifestRestore(ctx, lease, owner, dbPath, manifest); err != nil {
		return err
	}
	for _, artifact := range slices.Backward(manifest.Artifacts) {
		original := dbPath + artifact.Suffix
		backup := original + ".clear-backup"
		backupExists, err := regularFileExists(backup)
		if err != nil {
			return err
		}
		if !backupExists {
			current, err := verifySessionDBClearArtifact(ctx, lease, owner, original, artifact)
			if err != nil {
				return fmt.Errorf("session: verify unmoved clear artifact %q: %w", original, err)
			}
			if err := current.Close(); err != nil {
				return err
			}
			continue
		}

		boundBackup, err := verifySessionDBClearArtifact(ctx, lease, owner, backup, artifact)
		if err != nil {
			return fmt.Errorf("session: verify clear backup %q: %w", backup, err)
		}
		originalExists, err := regularFileExists(original)
		if err != nil {
			return errors.Join(err, boundBackup.Close())
		}
		if originalExists {
			current, bindErr := bindSessionDBClearArtifact(ctx, lease, owner, original, artifact.Suffix)
			if bindErr != nil {
				return errors.Join(bindErr, boundBackup.Close())
			}
			currentDigest, digestErr := current.SHA256()
			if digestErr != nil {
				return errors.Join(digestErr, current.Close(), boundBackup.Close())
			}
			if fmt.Sprintf("%x", currentDigest) == artifact.SHA256 {
				return errors.Join(
					fmt.Errorf(
						"%w: clear artifact exists at both original and backup",
						sessiondb.ErrSessionDBFamilyChanged,
					),
					current.Close(),
					boundBackup.Close(),
				)
			}
			if err := current.Remove(); err != nil {
				return errors.Join(err, boundBackup.Close())
			}
		}
		_, moveErr := boundBackup.MoveTo(original)
		if err := errors.Join(moveErr, boundBackup.Close()); err != nil {
			return err
		}
	}
	return nil
}

func discardSessionDBManifestBackups(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
	allowMissing bool,
) error {
	if err := manifest.requireOwner(owner); err != nil {
		return err
	}
	if err := requireNoUnexpectedSessionDBClearBackups(dbPath, manifest); err != nil {
		return err
	}
	if err := verifySessionDBManifestBackups(ctx, lease, owner, dbPath, manifest, allowMissing); err != nil {
		return err
	}
	for _, artifact := range manifest.Artifacts {
		backup := dbPath + artifact.Suffix + ".clear-backup"
		exists, err := regularFileExists(backup)
		if err != nil {
			return err
		}
		if !exists {
			if allowMissing {
				continue
			}
			return fmt.Errorf("%w: expected clear backup %q is missing", sessiondb.ErrSessionDBFamilyChanged, backup)
		}
		bound, err := verifySessionDBClearArtifact(ctx, lease, owner, backup, artifact)
		if err != nil {
			return err
		}
		if err := bound.Remove(); err != nil {
			return err
		}
	}
	return nil
}

func verifySessionDBManifestBackups(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
	allowMissing bool,
) error {
	for _, artifact := range manifest.Artifacts {
		backup := dbPath + artifact.Suffix + ".clear-backup"
		exists, err := regularFileExists(backup)
		if err != nil {
			return err
		}
		if !exists {
			if allowMissing {
				continue
			}
			return fmt.Errorf("%w: expected clear backup %q is missing", sessiondb.ErrSessionDBFamilyChanged, backup)
		}
		bound, err := verifySessionDBClearArtifact(ctx, lease, owner, backup, artifact)
		if err != nil {
			return err
		}
		if err := bound.Close(); err != nil {
			return err
		}
	}
	return nil
}

func preflightSessionDBManifestRestore(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
) error {
	for _, artifact := range manifest.Artifacts {
		original := dbPath + artifact.Suffix
		backup := original + ".clear-backup"
		backupExists, err := regularFileExists(backup)
		if err != nil {
			return err
		}
		if !backupExists {
			current, err := verifySessionDBClearArtifact(ctx, lease, owner, original, artifact)
			if err != nil {
				return err
			}
			if err := current.Close(); err != nil {
				return err
			}
			continue
		}
		boundBackup, err := verifySessionDBClearArtifact(ctx, lease, owner, backup, artifact)
		if err != nil {
			return err
		}
		if err := boundBackup.Close(); err != nil {
			return err
		}
		originalExists, err := regularFileExists(original)
		if err != nil || !originalExists {
			if err != nil {
				return err
			}
			continue
		}
		current, err := bindSessionDBClearArtifact(ctx, lease, owner, original, artifact.Suffix)
		if err != nil {
			return err
		}
		currentDigest, digestErr := current.SHA256()
		closeErr := current.Close()
		if digestErr != nil || closeErr != nil {
			return errors.Join(digestErr, closeErr)
		}
		if fmt.Sprintf("%x", currentDigest) == artifact.SHA256 {
			return fmt.Errorf(
				"%w: clear artifact exists at both original and backup",
				sessiondb.ErrSessionDBFamilyChanged,
			)
		}
	}
	return nil
}

func requireNoUnexpectedSessionDBClearBackups(path string, manifest sessionDBClearManifest) error {
	for _, suffix := range sessionDBClearArtifactSuffixes {
		backup := path + suffix + ".clear-backup"
		exists, err := regularFileExists(backup)
		if err != nil {
			return err
		}
		if exists && !manifest.hasArtifact(suffix) {
			return fmt.Errorf("%w: unexpected clear backup %q", sessiondb.ErrSessionDBFamilyChanged, backup)
		}
	}
	return nil
}

func requireSessionDBClearBackupState(
	path string,
	manifest sessionDBClearManifest,
	moved []sessionDBClearArtifact,
) error {
	movedSuffixes := make(map[string]struct{}, len(moved))
	for _, artifact := range moved {
		movedSuffixes[artifact.Suffix] = struct{}{}
	}
	for _, suffix := range sessionDBClearArtifactSuffixes {
		originalExists, err := regularFileExists(path + suffix)
		if err != nil {
			return err
		}
		backupExists, err := regularFileExists(path + suffix + ".clear-backup")
		if err != nil {
			return err
		}
		expected := manifest.hasArtifact(suffix)
		_, alreadyMoved := movedSuffixes[suffix]
		wantOriginal := expected && !alreadyMoved
		wantBackup := expected && alreadyMoved
		if originalExists != wantOriginal || backupExists != wantBackup {
			return fmt.Errorf(
				"%w: clear family suffix %q has original=%t backup=%t, want original=%t backup=%t",
				sessiondb.ErrSessionDBFamilyChanged,
				suffix,
				originalExists,
				backupExists,
				wantOriginal,
				wantBackup,
			)
		}
	}
	return nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("session: inspect database artifact %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("session: database artifact %q is not a regular file", path)
	}
	return true, nil
}

func (m *Manager) recoverSessionDBClear(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	dbPath string,
) error {
	manifest, present, err := readSessionDBClearManifest(dbPath)
	if err != nil {
		return err
	}
	marker, markerPresent, err := readSessionDBClearCommit(dbPath)
	if err != nil {
		return err
	}
	backupsPresent, err := sessionDBClearBackupsPresent(dbPath)
	if err != nil {
		return err
	}
	if !present {
		if markerPresent || backupsPresent {
			return fmt.Errorf(
				"%w: clear recovery state has no generation manifest",
				sessiondb.ErrSessionDBFamilyChanged,
			)
		}
		return nil
	}
	if err := manifest.requireOwner(owner); err != nil {
		return err
	}
	commit, committed, err := resolveSessionDBClearCommit(manifest, marker, markerPresent)
	if err != nil {
		return err
	}
	if committed {
		if err := verifySessionDBManifestBackups(ctx, lease, owner, dbPath, manifest, true); err != nil {
			return err
		}
		if err := m.reconcileSessionDBClearEpoch(ctx, owner.SessionID, commit.TranscriptEpoch); err != nil {
			return err
		}
		if err := m.discardOwnedMaterializedSessionLedger(ctx, owner, dbPath); err != nil {
			return err
		}
		if err := discardSessionDBManifestBackups(ctx, lease, owner, dbPath, manifest, true); err != nil {
			return fmt.Errorf("session: discard committed clear backups: %w", err)
		}
	} else if err := restoreSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest); err != nil {
		return fmt.Errorf("session: restore interrupted clear backups: %w", err)
	}
	if err := discardSessionDBClearCommit(dbPath); err != nil {
		return err
	}
	return discardSessionDBClearManifest(dbPath)
}

func sessionDBClearBackupsPresent(dbPath string) (bool, error) {
	for _, suffix := range sessionDBClearArtifactSuffixes {
		exists, err := regularFileExists(dbPath + suffix + ".clear-backup")
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}
