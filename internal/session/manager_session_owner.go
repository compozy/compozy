package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// SessionOwner returns the scope that owns a session, proven against the catalog,
// without repairing its metadata: resolving where a session lives never writes it (a
// continue/fork source stays byte-for-byte unchanged, ADR-007).
func (m *Manager) SessionOwner(ctx context.Context, sessionID string) (store.SessionOwner, error) {
	if m == nil {
		return store.SessionOwner{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return store.SessionOwner{}, errors.New("session: session owner context is required")
	}
	meta, err := m.readSessionMetaReadOnly(ctx, sessionID)
	if err != nil {
		return store.SessionOwner{}, err
	}
	databaseOwner, err := meta.DatabaseOwner()
	if err != nil {
		return store.SessionOwner{}, err
	}
	return store.SessionOwner{
		SessionID: meta.ID, ProfileID: meta.ProfileID, WorkspaceID: meta.WorkspaceID,
		DatabaseOwner: databaseOwner,
	}, nil
}

func (m *Manager) resolveStoredSessionOwner(
	ctx context.Context,
	meta *store.SessionMeta,
) (store.SessionDBOwner, error) {
	metadataOwner, err := meta.DatabaseOwner()
	if err != nil {
		return store.SessionDBOwner{}, err
	}
	if m == nil || m.sessionCatalog == nil {
		return metadataOwner, nil
	}
	catalogOwner, err := store.LookupSessionOwner(ctx, m.sessionCatalog, m.creationStore, metadataOwner.SessionID)
	if err != nil {
		return store.SessionDBOwner{}, err
	}
	if err := catalogOwner.BindMetadata(meta); err != nil {
		return store.SessionDBOwner{}, err
	}
	return catalogOwner.DatabaseOwner, nil
}
