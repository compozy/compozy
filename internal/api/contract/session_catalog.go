package contract

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
