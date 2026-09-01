package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

func (m *Manager) acquireOwnedSessionDBFamily(
	ctx context.Context,
	owner store.SessionDBOwner,
	dbPath string,
	candidates ...string,
) (*sessiondb.FamilyLease, func(), error) {
	resumeQueries, err := m.quiesceSessionQueries(ctx, owner, filepath.Dir(dbPath))
	if err != nil {
		return nil, nil, err
	}
	resumeQueries = sync.OnceFunc(resumeQueries)

	lease, err := acquireVerifiedSessionDBFamilyLease(ctx, owner, dbPath, candidates...)
	if err != nil {
		resumeQueries()
		return nil, nil, err
	}
	return lease, resumeQueries, nil
}

func acquireVerifiedSessionDBFamilyLease(
	ctx context.Context,
	owner store.SessionDBOwner,
	dbPath string,
	candidates ...string,
) (*sessiondb.FamilyLease, error) {
	lease, err := sessiondb.AcquireFamilyLease(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := verifyOwnedSessionDBCandidates(ctx, lease, owner, candidates...); err != nil {
		lease.Release()
		return nil, err
	}
	return lease, nil
}

func verifyOwnedSessionDBCandidates(
	ctx context.Context,
	lease *sessiondb.FamilyLease,
	owner store.SessionDBOwner,
	candidates ...string,
) error {
	verified := false
	for _, candidate := range candidates {
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("session: inspect owned event store %q: %w", candidate, err)
		}
		if err := lease.VerifyOwner(ctx, owner, candidate); err != nil {
			return fmt.Errorf("session: verify owned event store %q: %w", candidate, err)
		}
		verified = true
	}
	if !verified {
		return fmt.Errorf("session: verify owned event store: %w", sessiondb.ErrSessionDBOwnerMissing)
	}
	return nil
}

func (m *Manager) recoverOwnedSessionDBClear(
	ctx context.Context,
	owner store.SessionDBOwner,
	dbPath string,
) error {
	if err := m.waitForConversationFinalization(ctx, owner.SessionID); err != nil {
		return err
	}
	pending, err := sessionDBClearRecoveryPending(dbPath)
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}
	lease, resumeQueries, err := m.acquireOwnedSessionDBFamily(
		ctx,
		owner,
		dbPath,
		dbPath,
		dbPath+".clear-backup",
	)
	if err != nil {
		return err
	}
	defer lease.Release()
	defer resumeQueries()

	return m.recoverSessionDBClear(ctx, lease, owner, dbPath)
}

func sessionDBClearRecoveryPending(dbPath string) (bool, error) {
	_, manifestPresent, err := readSessionDBClearManifest(dbPath)
	if err != nil {
		return false, err
	}
	_, committed, err := readSessionDBClearCommit(dbPath)
	if err != nil {
		return false, err
	}
	backupsPresent, err := sessionDBClearBackupsPresent(dbPath)
	if err != nil {
		return false, err
	}
	return manifestPresent || committed || backupsPresent, nil
}

func (m *Manager) reconcileSessionDBClearEpoch(ctx context.Context, sessionID string, transcriptEpoch int64) error {
	if transcriptEpoch <= 0 || m == nil || m.transcriptEpochStore == nil {
		return nil
	}
	if _, err := m.transcriptEpochStore.EnsureSessionTranscriptEpoch(ctx, store.SessionTranscriptEpochUpdate{
		SessionID: sessionID,
		Minimum:   transcriptEpoch,
	}); err != nil {
		return fmt.Errorf("session: reconcile transcript epoch for %q: %w", sessionID, err)
	}
	return nil
}
