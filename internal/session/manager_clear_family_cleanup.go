package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

func (m *Manager) finalizeCommittedSessionDBClear(
	ctx context.Context,
	owner store.SessionDBOwner,
	dbPath string,
	manifest sessionDBClearManifest,
) error {
	lease, err := acquireVerifiedSessionDBFamilyLease(
		ctx,
		owner,
		dbPath,
		dbPath,
		dbPath+".clear-backup",
	)
	if err != nil {
		return fmt.Errorf("session: verify committed clear family: %w", err)
	}
	defer lease.Release()

	currentManifest, present, err := readSessionDBClearManifest(dbPath)
	if err != nil {
		return err
	}
	if !present || currentManifest.Generation != manifest.Generation {
		return fmt.Errorf("%w: clear manifest generation changed", sessiondb.ErrSessionDBFamilyChanged)
	}
	if err := currentManifest.requireOwner(owner); err != nil {
		return err
	}
	marker, markerPresent, err := readSessionDBClearCommit(dbPath)
	if err != nil {
		return err
	}
	_, committed, err := resolveSessionDBClearCommit(currentManifest, marker, markerPresent)
	if err != nil {
		return err
	}
	if !committed {
		return fmt.Errorf("%w: clear commit does not match manifest", sessiondb.ErrSessionDBFamilyChanged)
	}
	if err := m.discardOwnedMaterializedSessionLedger(ctx, owner, dbPath); err != nil {
		return err
	}
	if err := discardSessionDBManifestBackups(ctx, lease, owner, dbPath, currentManifest, true); err != nil {
		return err
	}
	if err := discardSessionDBClearCommit(dbPath); err != nil {
		return err
	}
	return discardSessionDBClearManifest(dbPath)
}

func (m *Manager) restoreClearedConversationFailure(
	ctx context.Context,
	owner store.SessionDBOwner,
	manifest sessionDBClearManifest,
	dbPath string,
	metaPath string,
	meta store.SessionMeta,
) error {
	lease, err := acquireVerifiedSessionDBFamilyLease(
		ctx,
		owner,
		dbPath,
		dbPath,
		dbPath+".clear-backup",
	)
	if err != nil {
		return fmt.Errorf("session: verify failed clear family before restore: %w", err)
	}
	defer lease.Release()

	if err := restoreSessionDBManifestArtifacts(ctx, lease, owner, dbPath, manifest); err != nil {
		return err
	}
	var errs []error
	if err := discardSessionDBClearCommit(dbPath); err != nil {
		errs = append(errs, err)
	}
	metaRestored := true
	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		metaRestored = false
		errs = append(errs, fmt.Errorf("session: restore cleared metadata for %q: %w", meta.ID, err))
	}
	if err := discardSessionDBClearManifest(dbPath); err != nil {
		errs = append(errs, err)
	}
	if metaRestored {
		lease.Release()
		if err := m.rematerializeStoppedSessionLedger(ctx, meta.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
