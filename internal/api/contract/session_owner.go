package contract

// SessionOwner is the minimal catalog ownership projection for one session.
// Global history has an empty workspace_id and the workspace_name "Global".
type SessionOwner struct {
	SessionID     string `json:"session_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}
