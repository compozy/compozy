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
	if owner.WorkspaceID == "" {
		return SessionDBOwner{}, errors.New("store: session database owner workspace id is required")
	}
	return owner, nil
}

// DatabaseOwner preserves the immutable events.db identity after catalog scope
// migrates to Global. The catalog must still authorize this metadata witness.
func (m *SessionMeta) DatabaseOwner() (SessionDBOwner, error) {
	workspaceID := m.WorkspaceID
	if strings.TrimSpace(workspaceID) == "" && m.CreationProfile != nil {
		workspaceID = m.CreationProfile.WorkspaceID
	}
	return (SessionDBOwner{SessionID: m.ID, WorkspaceID: workspaceID}).Normalize()
}
