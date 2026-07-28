package workspace

import (
	"context"
	"time"
)

// CoordinationSettings owns the persisted workspace opt-in consulted by
// coordinated execution resolution. Public mutations go through
// CoordinationCommands so revision, provenance, events, and task intent stay
// in one command boundary.
type CoordinationSettings interface {
	Get(ctx context.Context, workspaceID string) (CoordinationSetting, error)
}

// CoordinationSetting is the workspace-scoped, revisioned coordination state.
type CoordinationSetting struct {
	WorkspaceID string    `json:"workspace_id"`
	ScopeKind   string    `json:"scope_kind"`
	ScopeID     string    `json:"scope_id"`
	Enabled     bool      `json:"enabled"`
	Revision    int64     `json:"revision"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}
