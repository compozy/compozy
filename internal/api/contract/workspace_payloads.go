package contract

import "time"

// CreateWorkspaceRequest is the shared workspace creation request payload.
type CreateWorkspaceRequest struct {
	RootDir      string   `json:"root_dir"`
	Name         string   `json:"name,omitempty"`
	AddDirs      []string `json:"add_dirs,omitempty"`
	DefaultAgent string   `json:"default_agent,omitempty"`
	SandboxRef   string   `json:"sandbox_ref,omitempty"`
}

// UpdateWorkspaceRequest is the shared workspace update request payload.
type UpdateWorkspaceRequest struct {
	Name         *string   `json:"name"`
	AddDirs      *[]string `json:"add_dirs"`
	DefaultAgent *string   `json:"default_agent"`
	SandboxRef   *string   `json:"sandbox_ref"`
}

// ResolveWorkspaceRequest is the shared workspace resolve request payload.
type ResolveWorkspaceRequest struct {
	Path string `json:"path"`
}

// WorkspacePayload is the shared workspace response payload.
type WorkspacePayload struct {
	ID           string    `json:"id"`
	RootDir      string    `json:"root_dir"`
	AddDirs      []string  `json:"add_dirs"`
	Name         string    `json:"name"`
	DefaultAgent string    `json:"default_agent,omitempty"`
	SandboxRef   string    `json:"sandbox_ref,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// WorkspaceSkillPayload is the shared workspace skill response payload.
type WorkspaceSkillPayload struct {
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Source string `json:"source"`
}
