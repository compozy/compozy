package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const workspaceFolderIDPrefix = "wfold"

// WorkspaceLayout returns one profile's workspace folders and placements.
func (m *Manager) WorkspaceLayout(ctx context.Context, profileID string) (store.WorkspaceLayout, error) {
	layout, err := m.workspaceLayoutStore()
	if err != nil {
		return store.WorkspaceLayout{}, err
	}
	return layout.WorkspaceLayout(ctx, profileID)
}

// CreateWorkspaceFolder adds a named root folder at the top of the folder order.
func (m *Manager) CreateWorkspaceFolder(
	ctx context.Context,
	profileID string,
	name string,
) (store.WorkspaceFolder, error) {
	layout, err := m.workspaceLayoutStore()
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	id, err := newID(workspaceFolderIDPrefix)
	if err != nil {
		return store.WorkspaceFolder{}, fmt.Errorf("session: generate workspace folder id: %w", err)
	}
	folder, err := layout.CreateWorkspaceFolder(ctx, profileID, id, name)
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	m.publishWorkspaceLayoutChanged(profileID)
	return folder, nil
}

// RenameWorkspaceFolder changes one folder's display name.
func (m *Manager) RenameWorkspaceFolder(
	ctx context.Context,
	profileID string,
	folderID string,
	name string,
) (store.WorkspaceFolder, error) {
	layout, err := m.workspaceLayoutStore()
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	folder, err := layout.RenameWorkspaceFolder(ctx, profileID, folderID, name)
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	m.publishWorkspaceLayoutChanged(profileID)
	return folder, nil
}

// DeleteWorkspaceFolder removes a folder and returns its workspaces to the root.
func (m *Manager) DeleteWorkspaceFolder(ctx context.Context, profileID string, folderID string) error {
	return m.mutateWorkspaceLayout(profileID, func(layout store.WorkspaceLayoutStore) error {
		return layout.DeleteWorkspaceFolder(ctx, profileID, folderID)
	})
}

// ReorderWorkspaceFolders applies a complete workspace folder order.
func (m *Manager) ReorderWorkspaceFolders(ctx context.Context, profileID string, folderIDs []string) error {
	return m.mutateWorkspaceLayout(profileID, func(layout store.WorkspaceLayoutStore) error {
		return layout.ReorderWorkspaceFolders(ctx, profileID, folderIDs)
	})
}

// MoveWorkspace moves a workspace to the end of a folder, or to the end of
// the root when folderID is empty.
func (m *Manager) MoveWorkspace(ctx context.Context, profileID string, workspaceID string, folderID string) error {
	return m.mutateWorkspaceLayout(profileID, func(layout store.WorkspaceLayoutStore) error {
		return layout.MoveWorkspace(ctx, profileID, workspaceID, folderID)
	})
}

// ReorderWorkspaces pins a manual workspace order inside one container.
func (m *Manager) ReorderWorkspaces(
	ctx context.Context,
	profileID string,
	folderID string,
	workspaceIDs []string,
) error {
	return m.mutateWorkspaceLayout(profileID, func(layout store.WorkspaceLayoutStore) error {
		return layout.ReorderWorkspaces(ctx, profileID, folderID, workspaceIDs)
	})
}

func (m *Manager) mutateWorkspaceLayout(profileID string, mutate func(store.WorkspaceLayoutStore) error) error {
	layout, err := m.workspaceLayoutStore()
	if err != nil {
		return err
	}
	if err := mutate(layout); err != nil {
		return err
	}
	m.publishWorkspaceLayoutChanged(profileID)
	return nil
}

func (m *Manager) workspaceLayoutStore() (store.WorkspaceLayoutStore, error) {
	if m == nil {
		return nil, fmt.Errorf("session: manager is required")
	}
	layout, ok := m.sessionCatalog.(store.WorkspaceLayoutStore)
	if !ok || layout == nil {
		return nil, ErrSessionLayoutUnavailable
	}
	return layout, nil
}

func (m *Manager) publishWorkspaceLayoutChanged(profileID string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	m.publishSessionCatalogEvent(CatalogEvent{
		Name:      CatalogEventNameChanged,
		Kind:      CatalogEventWorkspaceLayout,
		ProfileID: profileID,
	})
}
