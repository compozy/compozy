package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// ErrSessionLayoutUnavailable reports that layout persistence is not configured.
var ErrSessionLayoutUnavailable = errors.New("session: session layout store is unavailable")

const sessionFolderIDPrefix = "sfold"

// ListSessionFolders returns one profile's folders inside a workspace.
func (m *Manager) ListSessionFolders(
	ctx context.Context,
	scope store.SessionLayoutScope,
) ([]store.SessionFolder, error) {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return nil, err
	}
	return layout.ListSessionFolders(ctx, scope)
}

// CreateSessionFolder adds a named folder at the top of the folder order.
func (m *Manager) CreateSessionFolder(
	ctx context.Context,
	scope store.SessionLayoutScope,
	name string,
) (store.SessionFolder, error) {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return store.SessionFolder{}, err
	}
	id, err := newID(sessionFolderIDPrefix)
	if err != nil {
		return store.SessionFolder{}, fmt.Errorf("session: generate folder id: %w", err)
	}
	folder, err := layout.CreateSessionFolder(ctx, scope, id, name)
	if err != nil {
		return store.SessionFolder{}, err
	}
	m.publishSessionLayoutChanged(scope)
	return folder, nil
}

// RenameSessionFolder changes one folder's display name.
func (m *Manager) RenameSessionFolder(
	ctx context.Context,
	scope store.SessionLayoutScope,
	folderID string,
	name string,
) (store.SessionFolder, error) {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return store.SessionFolder{}, err
	}
	folder, err := layout.RenameSessionFolder(ctx, scope, folderID, name)
	if err != nil {
		return store.SessionFolder{}, err
	}
	m.publishSessionLayoutChanged(scope)
	return folder, nil
}

// DeleteSessionFolder removes a folder and returns its sessions to the root.
func (m *Manager) DeleteSessionFolder(ctx context.Context, scope store.SessionLayoutScope, folderID string) error {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return err
	}
	if err := layout.DeleteSessionFolder(ctx, scope, folderID); err != nil {
		return err
	}
	m.publishSessionLayoutChanged(scope)
	return nil
}

// ReorderSessionFolders applies a complete folder order.
func (m *Manager) ReorderSessionFolders(
	ctx context.Context,
	scope store.SessionLayoutScope,
	folderIDs []string,
) ([]store.SessionFolder, error) {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return nil, err
	}
	folders, err := layout.ReorderSessionFolders(ctx, scope, folderIDs)
	if err != nil {
		return nil, err
	}
	m.publishSessionLayoutChanged(scope)
	return folders, nil
}

// MoveSession moves a session to the end of a folder, or to the root when
// folderID is empty.
func (m *Manager) MoveSession(
	ctx context.Context,
	scope store.SessionLayoutScope,
	sessionID string,
	folderID string,
) error {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return err
	}
	if err := layout.MoveSession(ctx, scope, sessionID, folderID); err != nil {
		return err
	}
	m.publishSessionLayoutChanged(scope)
	return nil
}

// ReorderSessions pins a manual session order inside one container.
func (m *Manager) ReorderSessions(
	ctx context.Context,
	scope store.SessionLayoutScope,
	folderID string,
	sessionIDs []string,
) error {
	layout, err := m.sessionLayoutStore()
	if err != nil {
		return err
	}
	if err := layout.ReorderSessions(ctx, scope, folderID, sessionIDs); err != nil {
		return err
	}
	m.publishSessionLayoutChanged(scope)
	return nil
}

func (m *Manager) sessionLayoutStore() (store.SessionLayoutStore, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	layout, ok := m.sessionCatalog.(store.SessionLayoutStore)
	if !ok || layout == nil {
		return nil, ErrSessionLayoutUnavailable
	}
	return layout, nil
}

func (m *Manager) publishSessionLayoutChanged(scope store.SessionLayoutScope) {
	normalized, err := scope.Normalize()
	if err != nil {
		return
	}
	m.publishSessionCatalogEvent(CatalogEvent{
		Name:        CatalogEventNameChanged,
		Kind:        CatalogEventLayout,
		ProfileID:   normalized.ProfileID,
		WorkspaceID: normalized.WorkspaceID,
	})
}
