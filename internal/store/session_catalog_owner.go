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

// SessionOwner separates current catalog scope from immutable events.db ownership.
// A migrated Global session retains its original database owner in its creation witness.
type SessionOwner struct {
	SessionID        string
	ProfileID        string
	WorkspaceID      string
	DatabaseOwner    SessionDBOwner
	CreationIdentity *SessionCreationIdentity
}

// BindMetadata proves the immutable owner before applying migrated catalog scope
// to an in-memory read. It does not rewrite metadata or its creation witness.
func (o SessionOwner) BindMetadata(meta *SessionMeta) error {
	if meta == nil {
		return errors.New("store: session metadata is required to prove ownership")
	}
	metadataOwner, err := meta.DatabaseOwner()
	if err != nil {
		return err
	}
	if metadataOwner != o.DatabaseOwner {
		return fmt.Errorf("%w: metadata owner %+v does not match catalog owner %+v",
			ErrSessionWorkspaceMismatch, metadataOwner, o.DatabaseOwner)
	}
	if o.CreationIdentity != nil && *o.CreationIdentity != (SessionCreationIdentity{
		CreationProfileRef: meta.CreationProfileRef,
		PolicySpecDigest:   meta.PolicySpecDigest,
		CreationDigest:     meta.CreationDigest,
	}) {
		return fmt.Errorf("%w: %s", ErrSessionCreationIdentityMismatch, meta.ID)
	}
	if meta.ProfileID != "" && meta.ProfileID != o.ProfileID {
		return fmt.Errorf("%w: metadata profile does not match catalog profile", ErrSessionWorkspaceMismatch)
	}
	meta.WorkspaceID = o.WorkspaceID
	meta.ProfileID = o.ProfileID
	return nil
}

// LookupSessionOwner proves current scope and database ownership from the durable
// catalog. Global history requires its retained, content-verified creation witness.
func LookupSessionOwner(
	ctx context.Context,
	catalog SessionCatalogOwnerReader,
	creations SessionCreationStore,
	sessionID string,
) (SessionOwner, error) {
	if ctx == nil {
		return SessionOwner{}, errors.New("store: session catalog owner context is required")
	}
	if catalog == nil {
		return SessionOwner{}, errors.New("store: session catalog is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return SessionOwner{}, errors.New("store: session id is required")
	}
	entries, err := catalog.ListSessions(ctx, SessionListQuery{
		ReadScope: ReadScope{AllProfiles: true},
		ID:        target,
		Limit:     2,
	})
	if err != nil {
		return SessionOwner{}, fmt.Errorf("store: read catalog owner for session %q: %w", target, err)
	}

	var resolved SessionOwner
	found := false
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) != target {
			continue
		}
		owner := SessionOwner{
			SessionID: target, ProfileID: strings.TrimSpace(entry.ProfileID),
			WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		}
		if found && (owner.WorkspaceID != resolved.WorkspaceID || owner.ProfileID != resolved.ProfileID) {
			return SessionOwner{}, fmt.Errorf("%w: %s", ErrSessionWorkspaceMismatch, target)
		}
		resolved = owner
		found = true
	}
	if !found {
		return SessionOwner{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	databaseWorkspaceID := resolved.WorkspaceID
	if databaseWorkspaceID == "" {
		if creations == nil {
			return SessionOwner{}, errors.New("store: Global session ownership requires the creation catalog")
		}
		identity, err := creations.GetSessionCreationIdentity(ctx, target)
		if err != nil {
			return SessionOwner{}, fmt.Errorf("store: read Global session creation identity: %w", err)
		}
		profile, err := creations.GetSessionCreationProfile(ctx, identity.CreationProfileRef)
		if err != nil {
			return SessionOwner{}, fmt.Errorf("store: read Global session creation profile: %w", err)
		}
		databaseWorkspaceID = profile.WorkspaceID
		resolved.CreationIdentity = &identity
	}
	resolved.DatabaseOwner, err = (SessionDBOwner{SessionID: target, WorkspaceID: databaseWorkspaceID}).Normalize()
	if err != nil {
		return SessionOwner{}, fmt.Errorf("store: normalize catalog database owner for session %q: %w", target, err)
	}
	return resolved, nil
}
