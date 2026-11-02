package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrSessionArchived reports that a runtime action requires restoring the session first.
	ErrSessionArchived = errors.New("session: session is archived")
	// ErrSessionArchiveRequiresStopped reports an archive request for a live session.
	ErrSessionArchiveRequiresStopped = errors.New("session: session must be stopped before archiving")
	// ErrSessionArchiveUnavailable reports that archive persistence is not configured.
	ErrSessionArchiveUnavailable = errors.New("session: archive store is unavailable")
	// ErrSessionArchiveStopFailed reports that archive-with-stop could not stop
	// the session, which therefore stays unarchived.
	ErrSessionArchiveStopFailed = errors.New("session: stop before archiving failed")
)

// Archive removes a stopped session from the default catalog without deleting
// its history or attachments, which remain readable after unarchive.
func (m *Manager) Archive(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	return m.setArchived(ctx, workspaceID, sessionID, true)
}

// archiveRequiresStoppedError keeps the public refusal text while naming the
// archive-family member whose live state blocked the archive.
type archiveRequiresStoppedError struct {
	sessionID string
	member    string
}

func (e *archiveRequiresStoppedError) Error() string {
	return ErrSessionArchiveRequiresStopped.Error() + ": " + e.sessionID
}

func (e *archiveRequiresStoppedError) Unwrap() error {
	return ErrSessionArchiveRequiresStopped
}

func newArchiveRequiresStoppedError(sessionID string, cause error) error {
	member := sessionID
	if live, ok := errors.AsType[*store.SessionArchiveLiveMemberError](cause); ok && live.SessionID != "" {
		member = live.SessionID
	}
	return &archiveRequiresStoppedError{sessionID: sessionID, member: member}
}

// ArchiveWithStop archives a session in one daemon-owned operation: every live
// member of its archive family is stopped through the normal user stop path,
// which cancels active turns and cascades to subagents, and the family is then
// archived. Completed subagents whose sessions the reaper has not stopped yet
// are stopped here too. A stopped family archives exactly as Archive does. A
// failed stop leaves the session unarchived and reports ErrSessionArchiveStopFailed.
func (m *Manager) ArchiveWithStop(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	sessionID = strings.TrimSpace(sessionID)
	stopped := make(map[string]bool)
	for {
		info, err := m.Archive(ctx, workspaceID, sessionID)
		refusal, ok := errors.AsType[*archiveRequiresStoppedError](err)
		if !ok {
			return info, err
		}
		// Each member is stopped at most once, so a member that stays live after
		// its stop fails the request instead of looping.
		if stopped[refusal.member] {
			return nil, fmt.Errorf("%w: %s: %w", ErrSessionArchiveStopFailed, sessionID, err)
		}
		stopped[refusal.member] = true
		// A member that finalized after the archive attempt is no longer live, so
		// its missing runtime is not a stop failure; the next attempt decides.
		if stopErr := m.Stop(ctx, refusal.member); stopErr != nil && !errors.Is(stopErr, ErrSessionNotFound) {
			return nil, fmt.Errorf("%w: %s: %w", ErrSessionArchiveStopFailed, sessionID, stopErr)
		}
	}
}

// Unarchive restores a session to the default catalog.
func (m *Manager) Unarchive(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	return m.setArchived(ctx, workspaceID, sessionID, false)
}

func (m *Manager) setArchived(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: archive context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("session: archive workspace and session ids are required")
	}
	archiveStore, ok := m.sessionCatalog.(store.SessionArchiveStore)
	if !ok || archiveStore == nil {
		return nil, ErrSessionArchiveUnavailable
	}
	stored, err := m.setSessionArchivedWithResumeGuard(
		ctx,
		archiveStore,
		workspaceID,
		sessionID,
		archived,
	)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSessionArchiveRequiresStopped):
			return nil, newArchiveRequiresStoppedError(sessionID, err)
		case errors.Is(err, store.ErrSessionArchived):
			return nil, fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
		default:
			return nil, err
		}
	}
	if archived {
		if service := m.ReplyWatches(); service != nil {
			service.OnTargetGone(ctx, sessionID)
			if err := service.OnSenderGone(ctx, sessionID); err != nil {
				return nil, err
			}
		}
	}
	info := sessionInfoFromCatalog(&stored)
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return info, nil
}

func (m *Manager) setSessionArchivedWithResumeGuard(
	ctx context.Context,
	archiveStore store.SessionArchiveStore,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	if !archived {
		return archiveStore.SetSessionArchived(ctx, workspaceID, sessionID, false)
	}

	if err := m.guardInstanceMain(ctx, sessionID); err != nil {
		return store.SessionInfo{}, err
	}
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if m.resumeLifecycle.runs[sessionID] != nil {
		return store.SessionInfo{}, newArchiveRequiresStoppedError(sessionID, nil)
	}
	return archiveStore.SetSessionArchived(ctx, workspaceID, sessionID, true)
}

// populateCatalogMetadata fills the archive and Later state of a stopped session.
func (m *Manager) populateCatalogMetadata(ctx context.Context, info *Info) error {
	if info == nil || info.State != StateStopped {
		return nil
	}
	archiveStore, ok := m.sessionCatalog.(store.SessionArchiveStore)
	if !ok || archiveStore == nil {
		return nil
	}
	archivedAt, err := archiveStore.SessionArchivedAt(ctx, info.WorkspaceID, info.ID)
	if err != nil {
		return fmt.Errorf("session: read archive metadata for %q: %w", info.ID, err)
	}
	info.ArchivedAt = cloneTimePointer(archivedAt)
	return m.populateLaterMetadata(ctx, info)
}

func (m *Manager) requireSessionUnarchived(ctx context.Context, workspaceID string, sessionID string) error {
	archiveStore, ok := m.sessionCatalog.(store.SessionArchiveStore)
	if !ok || archiveStore == nil {
		return nil
	}
	archivedAt, err := archiveStore.SessionArchivedAt(ctx, workspaceID, sessionID)
	if err != nil {
		return fmt.Errorf("session: read archive metadata for %q: %w", sessionID, err)
	}
	if archivedAt != nil {
		return fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
	}
	return nil
}
