package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

type sessionDBClearDisposition uint8

const (
	sessionDBClearRollback sessionDBClearDisposition = iota
	sessionDBClearPreserveCommittedRecovery
	sessionDBClearFinalizeCommitted
)

func (m *Manager) clearStoppedConversation(
	ctx context.Context,
	target string,
	owner store.SessionDBOwner,
	meta store.SessionMeta,
	sanitized store.SessionMeta,
	spec *sessionStartSpec,
) (_ *Session, err error) {
	clearDisposition := sessionDBClearRollback
	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, target))
	familyLease, resumeQueries, err := m.acquireOwnedSessionDBFamily(
		ctx,
		owner,
		dbPath,
		dbPath,
		dbPath+".clear-backup",
	)
	if err != nil {
		return nil, fmt.Errorf("session: acquire owned event store for clear %q: %w", target, err)
	}
	defer resumeQueries()
	defer familyLease.Release()

	manifest, err := m.backupSessionDBForClear(ctx, familyLease, owner, dbPath, target)
	if err != nil {
		return nil, err
	}
	cleanup := sessionDBClearCleanup{
		manager:     m,
		familyLease: familyLease,
		owner:       owner,
		dbPath:      dbPath,
		metaPath:    store.SessionMetaFile(filepath.Dir(dbPath)),
		manifest:    manifest,
		meta:        meta,
	}

	defer func() {
		err = errors.Join(err, cleanup.run(ctx, clearDisposition))
	}()

	if err := store.WriteSessionMeta(cleanup.metaPath, sanitized); err != nil {
		return nil, fmt.Errorf("session: persist cleared metadata for %q: %w", target, err)
	}
	if err := requireSessionDBClearBackupState(dbPath, manifest, manifest.Artifacts); err != nil {
		return nil, fmt.Errorf("session: verify clear family before replacement publication: %w", err)
	}
	if err := familyLease.PublishFreshOwnedDatabase(ctx, owner, dbPath); err != nil {
		return nil, fmt.Errorf("session: publish fresh event store for %q: %w", target, err)
	}
	familyLease.Release()

	restarted, err := m.restartClearedConversation(ctx, spec, target, dbPath, manifest.Generation)
	if err != nil {
		return nil, err
	}
	clearDisposition = sessionDBClearPreserveCommittedRecovery
	if err := m.discardOwnedMaterializedSessionLedger(ctx, owner, dbPath); err != nil {
		return restarted, err
	}
	clearDisposition = sessionDBClearFinalizeCommitted
	return restarted, nil
}
