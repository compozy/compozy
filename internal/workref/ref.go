// Package workref provides tiny shared workspace reference value objects used to
// pass workspace identifiers and paths through transport and runtime helpers.
package workref

import "strings"

// PathRef identifies one workspace by id plus transport-facing filesystem path.
type PathRef struct {
	WorkspaceID   string `json:"workspace_id,omitempty"   yaml:"workspace_id,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty" yaml:"workspace_path,omitempty"`
}

// RootRef identifies one workspace by id plus runtime/root-directory path.
type RootRef struct {
	WorkspaceID string `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
	Workspace   string `json:"workspace,omitempty"    yaml:"workspace,omitempty"`
}

// NewPath constructs one normalized transport-facing workspace reference.
func NewPath(id string, path string) PathRef {
	return PathRef{
		WorkspaceID:   strings.TrimSpace(id),
		WorkspacePath: strings.TrimSpace(path),
	}
}

// NewRoot constructs one normalized runtime-facing workspace reference.
func NewRoot(id string, root string) RootRef {
	return RootRef{
		WorkspaceID: strings.TrimSpace(id),
		Workspace:   strings.TrimSpace(root),
	}
}
