package session

import (
	"context"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
)

var (
	// ErrSoulRefreshConflict reports that a Soul refresh cannot run because
	// another session-scoped operation or an active task run is in progress.
	ErrSoulRefreshConflict = errors.New("session: soul refresh conflict")
	// ErrSoulRefreshDigestConflict reports a stale body-level expected digest.
	ErrSoulRefreshDigestConflict = errors.New("session: soul refresh expected digest conflict")
)

// SoulSnapshotStore is the durable storage needed by session Soul integration.
type SoulSnapshotStore interface {
	UpsertSoulSnapshot(ctx context.Context, snapshot soul.Snapshot) (soul.Snapshot, error)
	GetSoulSnapshot(ctx context.Context, id string) (soul.Snapshot, error)
	UpdateSessionSoulSnapshot(ctx context.Context, update store.SessionSoulSnapshotUpdate) error
}

// SoulRunActivityChecker reports whether a session currently owns an active run.
type SoulRunActivityChecker interface {
	HasActiveRunForSession(ctx context.Context, sessionID string, now time.Time) (bool, error)
}

// SoulRefreshResult is the internal result returned after a session Soul refresh.
type SoulRefreshResult struct {
	SessionID        string
	AgentName        string
	SoulSnapshotID   string
	SoulDigest       string
	ParentSoulDigest string
	Snapshot         *soul.Snapshot
	Soul             *soul.ResolvedSoul
	ConfigProvenance soul.ConfigProvenance
	RefreshedAt      time.Time
}

// RefreshSoul explicitly refreshes the session's resolved Soul snapshot.
func (m *Manager) RefreshSoul(ctx context.Context, id string) (SoulRefreshResult, error) {
	return m.RefreshSoulWithExpectedDigest(ctx, id, "")
}

// RefreshSoulWithExpectedDigest explicitly refreshes a session Soul snapshot after service-owned CAS.
func (m *Manager) RefreshSoulWithExpectedDigest(
	ctx context.Context,
	id string,
	expectedDigest string,
) (SoulRefreshResult, error) {
	if m == nil {
		return SoulRefreshResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return SoulRefreshResult{}, errors.New("session: soul refresh context is required")
	}
	session, err := m.lookup(id)
	if err != nil {
		return SoulRefreshResult{}, err
	}

	release, ok := m.tryAcquireSoulLock(session.ID)
	if !ok {
		return SoulRefreshResult{}, fmt.Errorf("%w: session %q soul lock is busy", ErrSoulRefreshConflict, session.ID)
	}
	defer release()

	info := session.Info()
	if info == nil {
		return SoulRefreshResult{}, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	if err := validateSoulRefreshExpectedDigest(info, expectedDigest); err != nil {
		return SoulRefreshResult{}, err
	}
	return m.refreshSoulLocked(ctx, session, info)
}

func (m *Manager) refreshSoulLocked(
	ctx context.Context,
	session *Session,
	info *Info,
) (SoulRefreshResult, error) {
	if err := m.ensureSoulRefreshAllowed(ctx, info, m.now()); err != nil {
		return SoulRefreshResult{}, err
	}

	workspaceSnapshot, artifacts, err := m.resolveSoulRefreshTarget(ctx, info)
	if err != nil {
		return SoulRefreshResult{}, err
	}
	resolved, err := m.resolveSoul(ctx, artifacts, &workspaceSnapshot)
	if err != nil {
		return SoulRefreshResult{}, fmt.Errorf("session: resolve soul for refresh %q: %w", info.ID, err)
	}

	durableCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.soulRefreshTimeout)
	defer cancel()

	refreshedAt := m.now().UTC()
	if active, activeErr := m.hasActiveSoulRun(durableCtx, info.ID, refreshedAt); activeErr != nil {
		return SoulRefreshResult{}, activeErr
	} else if active {
		return SoulRefreshResult{}, fmt.Errorf("%w: session %q has an active task run", ErrSoulRefreshConflict, info.ID)
	}

	snapshot, err := m.persistResolvedSoul(
		durableCtx,
		workspaceSnapshot.ID,
		info.AgentName,
		&resolved,
		workspaceSnapshot.Config.Agents.Soul,
		"session_refresh",
		refreshedAt,
	)
	if err != nil {
		return SoulRefreshResult{}, err
	}
	if err := m.storeSoulRefresh(durableCtx, session, info, snapshot, refreshedAt); err != nil {
		return SoulRefreshResult{}, err
	}
	provenance, err := soul.NewConfigProvenance(workspaceSnapshot.Config.Agents.Soul, "session_refresh")
	if err != nil {
		return SoulRefreshResult{}, err
	}
	return newSoulRefreshResult(info, snapshot, &resolved, provenance, refreshedAt), nil
}

func (m *Manager) ensureSoulRefreshAllowed(ctx context.Context, info *Info, now time.Time) error {
	if info.State != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, info.ID)
	}
	if active, activeErr := m.hasActiveSoulRun(ctx, info.ID, now); activeErr != nil {
		return activeErr
	} else if active {
		return fmt.Errorf("%w: session %q has an active task run", ErrSoulRefreshConflict, info.ID)
	}
	return nil
}

func (m *Manager) storeSoulRefresh(
	ctx context.Context,
	session *Session,
	info *Info,
	snapshot *soul.Snapshot,
	refreshedAt time.Time,
) error {
	update := store.SessionSoulSnapshotUpdate{
		ID:               info.ID,
		ParentSoulDigest: info.ParentSoulDigest,
		UpdatedAt:        refreshedAt,
	}
	if snapshot != nil {
		update.SoulSnapshotID = snapshot.ID
		update.SoulDigest = snapshot.Digest
	}
	if m.soulStore != nil {
		if err := m.soulStore.UpdateSessionSoulSnapshot(ctx, update); err != nil {
			return fmt.Errorf("session: update soul snapshot for %q: %w", info.ID, err)
		}
	}

	session.updateSoulSnapshot(snapshot, info.ParentSoulDigest, refreshedAt)
	if err := m.persistSessionMetadataOnly(session); err != nil {
		return err
	}
	return nil
}

func newSoulRefreshResult(
	info *Info,
	snapshot *soul.Snapshot,
	resolved *soul.ResolvedSoul,
	provenance soul.ConfigProvenance,
	refreshedAt time.Time,
) SoulRefreshResult {
	result := SoulRefreshResult{
		SessionID:        info.ID,
		AgentName:        info.AgentName,
		ParentSoulDigest: info.ParentSoulDigest,
		Snapshot:         cloneSoulSnapshotPointer(snapshot),
		Soul:             cloneResolvedSoulPointer(resolved),
		ConfigProvenance: provenance,
		RefreshedAt:      refreshedAt,
	}
	if snapshot != nil {
		result.SoulSnapshotID = snapshot.ID
		result.SoulDigest = snapshot.Digest
	}
	return result
}

func validateSoulRefreshExpectedDigest(info *Info, expectedDigest string) error {
	if info == nil {
		return errors.New("session: session info is required")
	}
	expected := strings.TrimSpace(expectedDigest)
	if expected == "" {
		return nil
	}
	current := strings.TrimSpace(info.SoulDigest)
	if current == "" {
		return fmt.Errorf("%w: expected_digest was provided but session %q has no Soul snapshot",
			ErrSoulRefreshDigestConflict,
			info.ID,
		)
	}
	if current != expected {
		return fmt.Errorf("%w: expected_digest does not match current session Soul digest",
			ErrSoulRefreshDigestConflict,
		)
	}
	return nil
}
