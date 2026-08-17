package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
)

// SessionCatalogResponse wraps one bounded public session catalog page.
type SessionCatalogResponse struct {
	Sessions []SessionPayload         `json:"sessions"`
	Page     CountedCursorPagePayload `json:"page"`
}

// SessionCatalogEventPayload is a workspace-identified wake signal. Clients must
// reconcile the authoritative catalog snapshot instead of counting events.
type SessionCatalogEventPayload struct {
	Kind        string `json:"kind"`
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SessionAttentionEventPayload is the post-commit attention edge carried by the catalog stream.
type SessionAttentionEventPayload struct {
	SessionID   string                 `json:"session_id"`
	WorkspaceID string                 `json:"workspace_id"`
	From        session.Badge          `json:"from"`
	To          session.Badge          `json:"to"`
	Class       session.AttentionClass `json:"class"`
	At          time.Time              `json:"at"`
}
