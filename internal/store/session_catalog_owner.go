package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SessionCatalogOwnerReader is the narrow durable-catalog read required for
// immutable owner authorization.
type SessionCatalogOwnerReader interface {
	ListSessions(ctx context.Context, query SessionListQuery) ([]SessionInfo, error)
}

// LookupSessionDBOwner resolves one session's immutable database owner from
// the durable catalog. It rejects missing and conflicting catalog rows.
func LookupSessionDBOwner(
	ctx context.Context,
	catalog SessionCatalogOwnerReader,
	sessionID string,
) (SessionDBOwner, error) {
	if ctx == nil {
		return SessionDBOwner{}, errors.New("store: session catalog owner context is required")
	}
	if catalog == nil {
		return SessionDBOwner{}, errors.New("store: session catalog is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return SessionDBOwner{}, errors.New("store: session id is required")
	}
	entries, err := catalog.ListSessions(ctx, SessionListQuery{
		ReadScope: ReadScope{AllProfiles: true},
		ID:        target,
		Limit:     2,
	})
	if err != nil {
		return SessionDBOwner{}, fmt.Errorf("store: read catalog owner for session %q: %w", target, err)
	}

	var resolved SessionDBOwner
	found := false
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) != target {
			continue
		}
		owner, err := (SessionDBOwner{SessionID: entry.ID, WorkspaceID: entry.WorkspaceID}).Normalize()
		if err != nil {
			return SessionDBOwner{}, fmt.Errorf("store: normalize catalog owner for session %q: %w", target, err)
		}
		if found && owner != resolved {
			return SessionDBOwner{}, fmt.Errorf("%w: %s", ErrSessionWorkspaceMismatch, target)
		}
		resolved = owner
		found = true
	}
	if !found {
		return SessionDBOwner{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	return resolved, nil
}
