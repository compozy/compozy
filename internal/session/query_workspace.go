package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// ListForWorkspace hydrates only sessions belonging to one registered workspace.
func (m *Manager) ListForWorkspace(ctx context.Context, workspaceID string) ([]*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: list context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("session: workspace id is required")
	}
	if m.sessionCatalog == nil {
		return nil, errors.New("session: session catalog is required")
	}
	activeByID := make(map[string]*Info)
	for _, info := range m.List() {
		if info != nil && info.Type != SessionTypeTriage && info.WorkspaceID == workspaceID {
			activeByID[info.ID] = info
		}
	}
	stored, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspaceID,
		Archive: store.SessionArchiveInclude,
	})
	if err != nil {
		return nil, fmt.Errorf("session: list workspace catalog: %w", err)
	}
	ids := make([]string, 0, len(stored))
	rows := make(sessionInventory, len(stored))
	for _, info := range stored {
		if info.SessionType == string(SessionTypeTriage) {
			continue
		}
		ids = append(ids, info.ID)
		rows[info.ID] = info
	}
	infos, err := m.mergePersistedSessionInfos(ctx, ids, activeByID, rows)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(infos, func(info *Info) bool { return info.Type == SessionTypeTriage || info.WorkspaceID != workspaceID }), nil
}

// WorkspaceResolver supplies runtime snapshots and inexpensive persisted registrations.
type WorkspaceResolver interface {
	workspacepkg.RuntimeResolver
	Get(context.Context, string) (workspacepkg.Workspace, error)
}

func (m *Manager) resolveWorkspaceRoot(ctx context.Context, workspaceID string) (string, error) {
	if ctx == nil {
		return "", nil
	}
	if strings.TrimSpace(workspaceID) == "" || m.workspace == nil {
		return "", nil
	}

	resolved, err := m.workspace.Get(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolved.RootDir), nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
