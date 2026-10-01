package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
)

// SessionCatalogResponse wraps one bounded public session catalog page.
type SessionCatalogResponse struct {
	Sessions []SessionPayload          `json:"sessions"`
	Page     SessionCatalogPagePayload `json:"page"`
}

// SessionCatalogPagePayload preserves exact totals by default. Clients may opt
// out with skip_total=true to read bounded cursor pages without a catalog count.
type SessionCatalogPagePayload struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Total      *int   `json:"total,omitempty"`
	Limit      int    `json:"limit"`
}

// SessionCatalogEventPayload is a workspace-identified wake signal. Clients must
// reconcile the authoritative catalog snapshot instead of counting events.
type SessionCatalogEventPayload struct {
	Kind        string `json:"kind"`
	ProfileID   string `json:"profile_id"`
	ProfileName string `json:"profile_name"`
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

// SessionAttentionEventPayload is the post-commit attention edge carried by the catalog stream.
type SessionAttentionEventPayload struct {
	SessionID   string                 `json:"session_id"`
	ProfileID   string                 `json:"profile_id"`
	ProfileName string                 `json:"profile_name"`
	WorkspaceID string                 `json:"workspace_id"`
	From        session.Badge          `json:"from"`
	To          session.Badge          `json:"to"`
	Class       session.AttentionClass `json:"class"`
	At          time.Time              `json:"at"`
}

// SessionCatalogFacetsPayload counts the entire eligible scope, independently
// of the selected badge and text query used to page visible rows.
type SessionCatalogFacetsPayload struct {
	All               int `json:"all"`
	NeedsYou          int `json:"needs_you"`
	Working           int `json:"working"`
	Finished          int `json:"finished"`
	Idle              int `json:"idle"`
	TerminalApprovals int `json:"terminal_approvals"`
}

type SessionWorkspaceFacetsPayload struct {
	WorkspaceID string                      `json:"workspace_id"`
	Facets      SessionCatalogFacetsPayload `json:"facets"`
}

type SessionCatalogFacetsResponse struct {
	Facets      SessionCatalogFacetsPayload     `json:"facets"`
	ByWorkspace []SessionWorkspaceFacetsPayload `json:"by_workspace"`
}
