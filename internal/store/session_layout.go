package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxSessionFolders bounds one profile's folders inside one workspace.
	MaxSessionFolders = 200
	// MaxSessionFolderNameLength bounds a folder display name in characters,
	// matching the schema's SQLite length() check and the Web form.
	MaxSessionFolderNameLength = 120
	// MaxSessionLayoutOrder bounds one manual reorder request.
	MaxSessionLayoutOrder = 500
	// MaxSessionPlacementLookup bounds one placement decoration read.
	MaxSessionPlacementLookup = 1000
)

var (
	// ErrSessionFolderNotFound reports a folder outside the caller's profile and workspace.
	ErrSessionFolderNotFound = errors.New("store: session folder not found")
	// ErrSessionFolderLimit reports that the scope already holds MaxSessionFolders folders.
	ErrSessionFolderLimit = errors.New("store: session folder limit reached")
	// ErrSessionLayoutInvalid reports a malformed layout mutation.
	ErrSessionLayoutInvalid = errors.New("store: session layout request is invalid")
	// ErrSessionLayoutConflict reports an order that does not match current layout state.
	ErrSessionLayoutConflict = errors.New("store: session layout changed concurrently")
	// ErrSessionLayoutSubagent reports a layout mutation for a subagent session,
	// which always follows its parent.
	ErrSessionLayoutSubagent = errors.New("store: subagent sessions follow their parent")
)

// SessionLayoutScope selects one profile's layout inside one workspace.
type SessionLayoutScope struct {
	ProfileID   string
	WorkspaceID string
}

// Normalize trims and validates the scope.
func (s SessionLayoutScope) Normalize() (SessionLayoutScope, error) {
	s.ProfileID = strings.TrimSpace(s.ProfileID)
	s.WorkspaceID = strings.TrimSpace(s.WorkspaceID)
	if s.ProfileID == "" {
		return SessionLayoutScope{}, fmt.Errorf("%w: profile and workspace are required", ErrSessionLayoutInvalid)
	}
	return s, nil
}

// SessionFolder is one named, ordered group of sessions.
type SessionFolder struct {
	ID           string
	ProfileID    string
	WorkspaceID  string
	Name         string
	Position     int
	SessionCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SessionPlacement is one session's folder membership and manual position.
// FolderID is empty for a session pinned in the workspace root.
type SessionPlacement struct {
	SessionID      string
	FolderID       string
	Position       int
	FolderPosition int
}

// NormalizeSessionFolderName trims a folder name and enforces its bounds.
func NormalizeSessionFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: folder name is required", ErrSessionLayoutInvalid)
	}
	if utf8.RuneCountInString(name) > MaxSessionFolderNameLength {
		return "", fmt.Errorf(
			"%w: folder name exceeds %d characters",
			ErrSessionLayoutInvalid,
			MaxSessionFolderNameLength,
		)
	}
	return name, nil
}

// NormalizeSessionLayoutIDs trims, bounds, and rejects duplicate or blank IDs.
func NormalizeSessionLayoutIDs(ids []string, limit int) ([]string, error) {
	if len(ids) > limit {
		return nil, fmt.Errorf("%w: at most %d ids are allowed", ErrSessionLayoutInvalid, limit)
	}
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: ids must not be blank", ErrSessionLayoutInvalid)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: duplicate id %q", ErrSessionLayoutInvalid, id)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

// SessionLayoutStore persists session folders and manual session order.
type SessionLayoutStore interface {
	ListSessionFolders(ctx context.Context, scope SessionLayoutScope) ([]SessionFolder, error)
	CreateSessionFolder(ctx context.Context, scope SessionLayoutScope, id string, name string) (SessionFolder, error)
	RenameSessionFolder(ctx context.Context, scope SessionLayoutScope, id string, name string) (SessionFolder, error)
	DeleteSessionFolder(ctx context.Context, scope SessionLayoutScope, id string) error
	ReorderSessionFolders(ctx context.Context, scope SessionLayoutScope, ids []string) ([]SessionFolder, error)
	MoveSession(ctx context.Context, scope SessionLayoutScope, sessionID string, folderID string) error
	ReorderSessions(ctx context.Context, scope SessionLayoutScope, folderID string, sessionIDs []string) error
	SessionPlacements(ctx context.Context, sessionIDs []string) (map[string]SessionPlacement, error)
}
