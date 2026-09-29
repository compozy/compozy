package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// SessionOwner returns the workspace that owns a session, proven against the catalog,
// without repairing its metadata: resolving where a session lives never writes it (a
// continue/fork source stays byte-for-byte unchanged, ADR-007).
func (m *Manager) SessionOwner(ctx context.Context, sessionID string) (store.SessionDBOwner, error) {
	if m == nil {
		return store.SessionDBOwner{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return store.SessionDBOwner{}, errors.New("session: session owner context is required")
	}
	meta, err := m.readSessionMetaReadOnly(ctx, sessionID)
	if err != nil {
		return store.SessionDBOwner{}, err
	}
	return (store.SessionDBOwner{SessionID: meta.ID, WorkspaceID: meta.WorkspaceID}).Normalize()
}

func (m *Manager) resolveStoredSessionOwner(
	ctx context.Context,
	sessionID string,
	metadataWorkspaceID string,
) (store.SessionDBOwner, error) {
	metadataOwner, err := (store.SessionDBOwner{
		SessionID:   sessionID,
		WorkspaceID: metadataWorkspaceID,
	}).Normalize()
	if err != nil {
		return store.SessionDBOwner{}, err
	}
	if m == nil || m.sessionCatalog == nil {
		return metadataOwner, nil
	}
	catalogOwner, err := store.LookupSessionDBOwner(ctx, m.sessionCatalog, metadataOwner.SessionID)
	if err != nil {
		return store.SessionDBOwner{}, err
	}
	if catalogOwner != metadataOwner {
		return store.SessionDBOwner{}, fmt.Errorf(
			"%w: metadata owner %+v does not match catalog owner %+v",
			store.ErrSessionWorkspaceMismatch,
			metadataOwner,
			catalogOwner,
		)
	}
	return catalogOwner, nil
}
