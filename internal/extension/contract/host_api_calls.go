package contract

// CallsListParams filters one profile-owned call page.
type CallsListParams struct {
	Scope       string   `json:"scope,omitempty"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
	State       []string `json:"state,omitempty"`
	Caller      string   `json:"caller,omitempty"`
	Cursor      string   `json:"cursor,omitempty"`
	Limit       int      `json:"limit,omitempty"`
}

// CallTargetParams identifies one call; omitted scope fields infer the bound workspace or default to global scope.
type CallTargetParams struct {
	CallID      string `json:"call_id"`
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// MessagesListParams filters one profile-owned mailbox page.
type MessagesListParams struct {
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}
