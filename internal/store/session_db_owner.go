package store

import (
	"errors"
	"strings"
)

// SessionDBOwner is the immutable identity authorized to access one events.db.
type SessionDBOwner struct {
	SessionID   string
	WorkspaceID string
}

// Normalize validates and canonicalizes a session database owner.
func (o SessionDBOwner) Normalize() (SessionDBOwner, error) {
	owner := SessionDBOwner{
		SessionID:   strings.TrimSpace(o.SessionID),
		WorkspaceID: strings.TrimSpace(o.WorkspaceID),
	}
	if owner.SessionID == "" {
		return SessionDBOwner{}, errors.New("store: session database owner session id is required")
	}
	return owner, nil
}
