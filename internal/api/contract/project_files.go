package contract

type FileRootPayload struct {
	Kind        string `json:"kind"`
	WorkspaceID string `json:"workspace_id"`
	WorktreeID  string `json:"worktree_id,omitempty"`
	Name        string `json:"name"`
	Branch      string `json:"branch,omitempty"`
}
type FileEntryPayload struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Kind            string `json:"kind"`
	Size            int64  `json:"size,omitzero"`
	Symlink         bool   `json:"symlink,omitzero"`
	Ignored         bool   `json:"ignored,omitzero"`
	Sensitive       bool   `json:"sensitive,omitzero"`
	GitStatus       string `json:"git_status,omitempty"`
	ContainsChanges bool   `json:"contains_changes,omitzero"`
}
type FileEntriesResponse struct {
	Root              FileRootPayload    `json:"root"`
	Path              string             `json:"path"`
	Entries           []FileEntryPayload `json:"entries"`
	Truncated         bool               `json:"truncated"`
	StatusUnavailable bool               `json:"status_unavailable,omitzero"`
}
type FileContentResponse struct {
	Root        FileRootPayload `json:"root"`
	Path        string          `json:"path"`
	Kind        string          `json:"kind"`
	Size        int64           `json:"size"`
	Language    string          `json:"language,omitempty"`
	Text        *string         `json:"text,omitempty"`
	ImageMIME   string          `json:"image_mime,omitempty"`
	ImageBase64 string          `json:"image_base64,omitempty"`
	Sensitive   bool            `json:"sensitive,omitzero"`
	Truncated   bool            `json:"truncated"`
	Revision    string          `json:"revision,omitempty"`
}
