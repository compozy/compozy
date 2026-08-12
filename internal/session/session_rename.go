package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

const SessionNameMaxRunes = 64

// Rename changes the durable display name of one user session without altering
// its runtime lifecycle or transcript identity.
func (m *Manager) Rename(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	name string,
) (*Info, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: rename context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: workspace id is required", ErrValidation)
	}
	target, err := normalizeStoredSessionID(sessionID)
	if err != nil {
		return nil, err
	}
	normalizedName, err := normalizeManualSessionName(name)
	if err != nil {
		return nil, err
	}

	if active, ok := m.Get(target); ok {
		return m.renameActiveSession(ctx, active, workspaceID, normalizedName)
	}
	return m.renameInactiveSession(ctx, target, workspaceID, normalizedName)
}

func normalizeManualSessionName(name string) (string, error) {
	normalized := strings.Join(strings.Fields(name), " ")
	if normalized == "" {
		return "", fmt.Errorf("%w: session name is required", ErrValidation)
	}
	if utf8.RuneCountInString(normalized) > SessionNameMaxRunes {
		return "", fmt.Errorf(
			"%w: session name must be at most %d characters",
			ErrValidation,
			SessionNameMaxRunes,
		)
	}
	return normalized, nil
}

func validateManualSessionRename(workspaceID string, sessionType Type, expectedWorkspaceID string) error {
	if strings.TrimSpace(workspaceID) != strings.TrimSpace(expectedWorkspaceID) {
		return ErrSessionNotFound
	}
	if normalizeSessionType(sessionType) != SessionTypeUser {
		return fmt.Errorf("%w: managed sessions cannot be renamed", ErrValidation)
	}
	return nil
}

func (m *Manager) renameActiveSession(
	ctx context.Context,
	active *Session,
	workspaceID string,
	name string,
) (*Info, error) {
	active.persistMu.Lock()
	defer active.persistMu.Unlock()

	active.mu.Lock()
	if err := validateManualSessionRename(workspaceID, active.Type, active.WorkspaceID); err != nil {
		active.mu.Unlock()
		return nil, err
	}
	if active.Name == name {
		info := active.infoLocked()
		active.mu.Unlock()
		return info, nil
	}

	previousMeta := active.metaLocked()
	active.Name = name
	active.UpdatedAt = m.now()
	meta := active.metaLocked()
	info := active.infoLocked()
	if err := m.persistSessionIdentitySnapshot(ctx, active.metaPath, meta, info); err != nil {
		active.Name = previousMeta.Name
		active.UpdatedAt = previousMeta.UpdatedAt
		if rollbackErr := store.WriteSessionMeta(active.metaPath, previousMeta); rollbackErr != nil {
			active.mu.Unlock()
			return nil, errors.Join(err, fmt.Errorf("session: roll back rename: %w", rollbackErr))
		}
		active.mu.Unlock()
		return nil, err
	}
	active.mu.Unlock()
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return info, nil
}

func (m *Manager) renameInactiveSession(
	ctx context.Context,
	target string,
	workspaceID string,
	name string,
) (*Info, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	if active, ok := m.Get(target); ok {
		return m.renameActiveSession(ctx, active, workspaceID, name)
	}
	if m.isPending(target) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, target)
	}
	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := validateManualSessionRename(workspaceID, Type(meta.SessionType), meta.WorkspaceID); err != nil {
		return nil, err
	}
	info := m.sessionInfoFromMeta(ctx, meta)
	if err := m.populateArchiveMetadata(ctx, info); err != nil {
		return nil, err
	}
	if meta.Name == name {
		return info, nil
	}

	previousMeta := meta
	meta.Name = name
	meta.UpdatedAt = m.now()
	info.Name = name
	info.UpdatedAt = meta.UpdatedAt
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		return nil, fmt.Errorf("session: persist rename for %q: %w", target, err)
	}
	if err := m.persistSessionCatalogSnapshot(ctx, meta, info); err != nil {
		if rollbackErr := store.WriteSessionMeta(metaPath, previousMeta); rollbackErr != nil {
			return nil, errors.Join(err, fmt.Errorf("session: roll back rename: %w", rollbackErr))
		}
		return nil, err
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return info, nil
}
