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
)

// Archive removes a stopped session from the default catalog without deleting its history.
func (m *Manager) Archive(ctx context.Context, workspaceID string, sessionID string) (*Info, error) {
	return m.setArchived(ctx, workspaceID, sessionID, true)
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
	if workspaceID == "" || sessionID == "" {
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
			return nil, fmt.Errorf("%w: %s", ErrSessionArchiveRequiresStopped, sessionID)
		case errors.Is(err, store.ErrSessionArchived):
			return nil, fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
		default:
			return nil, err
		}
	}
	info := sessionInfoFromCatalog(stored)
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

	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if m.resumeLifecycle.runs[sessionID] != nil {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", ErrSessionArchiveRequiresStopped, sessionID)
	}
	return archiveStore.SetSessionArchived(ctx, workspaceID, sessionID, true)
}

func (m *Manager) populateArchiveMetadata(ctx context.Context, info *Info) error {
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
	return nil
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
