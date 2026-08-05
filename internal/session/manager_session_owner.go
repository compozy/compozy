package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

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
