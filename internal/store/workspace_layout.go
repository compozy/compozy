package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// MaxWorkspaceFolders bounds one profile's root workspace folders.
	MaxWorkspaceFolders = 200
	// MaxWorkspaceLayoutOrder bounds one manual workspace reorder request.
	MaxWorkspaceLayoutOrder = 500
)

var (
	// ErrWorkspaceFolderNotFound reports a workspace folder outside the caller's profile.
	ErrWorkspaceFolderNotFound = errors.New("store: workspace folder not found")
	// ErrWorkspaceLayoutTargetNotFound reports a placement for an unregistered workspace.
	ErrWorkspaceLayoutTargetNotFound = errors.New("store: workspace not found")
	// ErrWorkspaceFolderLimit reports that the profile already holds MaxWorkspaceFolders folders.
	ErrWorkspaceFolderLimit = errors.New("store: workspace folder limit reached")
	// ErrWorkspaceLayoutInvalid reports a malformed workspace layout mutation.
	ErrWorkspaceLayoutInvalid = errors.New("store: workspace layout request is invalid")
	// ErrWorkspaceLayoutConflict reports an order that does not match current layout state.
	ErrWorkspaceLayoutConflict = errors.New("store: workspace layout changed concurrently")
)

// WorkspaceFolder is one named root folder that groups workspaces in one
// profile's All-projects sidebar.
type WorkspaceFolder struct {
	ID             string
	ProfileID      string
	Name           string
	Position       int
	WorkspaceCount int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WorkspacePlacement is one workspace's folder membership and manual
// position. WorkspaceID is empty for the User scope; FolderID is empty for a
// workspace pinned in the root.
type WorkspacePlacement struct {
	WorkspaceID string
	FolderID    string
	Position    int
}

// WorkspaceLayout is one profile's complete workspace layout: folders in
// order, then placements ordered by container (folders first, root last).
type WorkspaceLayout struct {
	Folders    []WorkspaceFolder
	Placements []WorkspacePlacement
}

// NormalizeWorkspaceLayoutProfile trims and requires the owning profile.
func NormalizeWorkspaceLayoutProfile(profileID string) (string, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return "", fmt.Errorf("%w: profile is required", ErrWorkspaceLayoutInvalid)
	}
	return profileID, nil
}

// WorkspaceLayoutStore persists root workspace folders and manual workspace order.
type WorkspaceLayoutStore interface {
	WorkspaceLayout(ctx context.Context, profileID string) (WorkspaceLayout, error)
	CreateWorkspaceFolder(ctx context.Context, profileID string, id string, name string) (WorkspaceFolder, error)
	RenameWorkspaceFolder(ctx context.Context, profileID string, id string, name string) (WorkspaceFolder, error)
	DeleteWorkspaceFolder(ctx context.Context, profileID string, id string) error
	ReorderWorkspaceFolders(ctx context.Context, profileID string, ids []string) error
	MoveWorkspace(ctx context.Context, profileID string, workspaceID string, folderID string) error
	ReorderWorkspaces(ctx context.Context, profileID string, folderID string, workspaceIDs []string) error
}
