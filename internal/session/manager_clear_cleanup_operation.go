package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

type sessionDBClearCleanup struct {
	manager     *Manager
	familyLease *sessiondb.FamilyLease
	owner       store.SessionDBOwner
	dbPath      string
	metaPath    string
	manifest    sessionDBClearManifest
	meta        store.SessionMeta
}

func (c *sessionDBClearCleanup) run(ctx context.Context, disposition sessionDBClearDisposition) error {
	if c.familyLease != nil {
		c.familyLease.Release()
	}
	if disposition == sessionDBClearPreserveCommittedRecovery {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()

	switch disposition {
	case sessionDBClearFinalizeCommitted:
		return c.manager.finalizeCommittedSessionDBClear(cleanupCtx, c.owner, c.dbPath, c.manifest)
	case sessionDBClearRollback:
		return c.manager.restoreClearedConversationFailure(
			cleanupCtx,
			c.owner,
			c.manifest,
			c.dbPath,
			c.metaPath,
			c.meta,
		)
	default:
		return errors.New("session: invalid clear cleanup disposition")
	}
}
