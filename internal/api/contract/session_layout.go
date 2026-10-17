package contract

import "time"

// SessionFolderPayload is one named group of sessions in a workspace sidebar.
type SessionFolderPayload struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProfileID   string `json:"profile_id"`
	Name        string `json:"name"`
	Position    int    `json:"position"`
	// SessionCount counts the folder's unarchived sessions.
	SessionCount int       `json:"session_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SessionFoldersResponse lists one profile's folders in a workspace in manual order.
type SessionFoldersResponse struct {
	Folders []SessionFolderPayload `json:"folders"`
}

// SessionFolderResponse returns one created or renamed folder.
type SessionFolderResponse struct {
	Folder SessionFolderPayload `json:"folder"`
}

// CreateSessionFolderRequest creates a folder at the top of the folder order.
type CreateSessionFolderRequest struct {
	Name string `json:"name"`
}

// RenameSessionFolderRequest changes a folder's display name.
type RenameSessionFolderRequest struct {
	Name string `json:"name"`
}

// ReorderSessionFoldersRequest lists every folder of the scope exactly once, in order.
type ReorderSessionFoldersRequest struct {
	FolderIDs []string `json:"folder_ids"`
}

// MoveSessionRequest moves one session to the end of a folder, or back to the
// workspace root when FolderID is empty.
type MoveSessionRequest struct {
	FolderID string `json:"folder_id,omitempty"`
}

// ReorderSessionsRequest pins a manual order inside one container: a folder,
// or the workspace root when FolderID is empty. Listed sessions move into the
// container; existing members that are not listed keep their order after them.
type ReorderSessionsRequest struct {
	FolderID   string   `json:"folder_id,omitempty"`
	SessionIDs []string `json:"session_ids"`
}
