package contract

import "time"

// WorkspaceFolderPayload is one named root folder that groups workspaces in
// one profile's All-projects sidebar.
type WorkspaceFolderPayload struct {
	ID        string `json:"id"`
	ProfileID string `json:"profile_id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	// WorkspaceCount counts the workspaces (and User scope) placed in the folder.
	WorkspaceCount int       `json:"workspace_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorkspacePlacementPayload pins one workspace into a folder or the root at a
// manual position. WorkspaceID is "@user" for the User scope; FolderID is
// omitted for the root. Workspaces without a placement follow the placed root
// workspaces in name order.
type WorkspacePlacementPayload struct {
	WorkspaceID string `json:"workspace_id"`
	FolderID    string `json:"folder_id,omitempty"`
	Position    int    `json:"position"`
}

// WorkspaceLayoutResponse is one profile's complete workspace layout: folders
// in manual order, then placements ordered by container (folders, then root).
type WorkspaceLayoutResponse struct {
	Folders    []WorkspaceFolderPayload    `json:"folders"`
	Placements []WorkspacePlacementPayload `json:"placements"`
}

// WorkspaceFolderResponse returns one created or renamed workspace folder.
type WorkspaceFolderResponse struct {
	Folder WorkspaceFolderPayload `json:"folder"`
}

// CreateWorkspaceFolderRequest creates a root folder at the top of the folder order.
type CreateWorkspaceFolderRequest struct {
	Name string `json:"name"`
}

// RenameWorkspaceFolderRequest changes a workspace folder's display name.
type RenameWorkspaceFolderRequest struct {
	Name string `json:"name"`
}

// ReorderWorkspaceFoldersRequest lists every workspace folder of the profile
// exactly once, in order.
type ReorderWorkspaceFoldersRequest struct {
	FolderIDs []string `json:"folder_ids"`
}

// MoveWorkspaceRequest moves one workspace (id, name, or "@user") to the end of
// a folder, or to the end of the root when FolderID is empty. A workspace
// already in that container keeps its position.
type MoveWorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
	FolderID    string `json:"folder_id,omitempty"`
}

// ReorderWorkspacesRequest pins a manual order inside one container: a
// folder, or the root when FolderID is empty. Listed workspaces move into the
// container; existing members that are not listed keep their order after them.
type ReorderWorkspacesRequest struct {
	FolderID     string   `json:"folder_id,omitempty"`
	WorkspaceIDs []string `json:"workspace_ids"`
}
