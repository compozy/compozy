package contract

// SessionOwner is the minimal catalog ownership projection for one session.
// User history has an empty workspace_id and the workspace_name "User".
type SessionOwner struct {
	SessionID     string `json:"session_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}
