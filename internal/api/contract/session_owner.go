package contract

// SessionOwner is the minimal workspace ownership projection for one session.
type SessionOwner struct {
	SessionID     string `json:"session_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}
