// Package extensionmcp defines instance-scoped MCP overrides and sticky runtime names.
package extensionmcp

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var (
	ErrNameTaken = errors.New("MCP server runtime name is already taken")
	ErrNotFound  = errors.New("extension MCP allocation not found")
)

// Target identifies the definition independently of its allocated runtime name.
type Target struct {
	Extension   string
	ProfileID   string
	WorkspaceID string
	ServerName  string
}

// Override contains the editable fields applied over a published declaration.
type Override struct {
	Env     map[string]string
	Headers map[string]string
	URL     string
}

// Record retains an allocation until the owning installed instance is removed.
type Record struct {
	Target
	Override
	RuntimeName string
	UpdatedAt   time.Time
}

// Store serializes allocation with the persisted names in one scope cell.
// occupied contains names from the effective registry, including manual definitions.
type Store interface {
	List(context.Context, string, string) ([]Record, error)
	ListAll(context.Context) ([]Record, error)
	Reserve(context.Context, Target, string, []string) (Record, error)
	Update(context.Context, Target, Override) error
	// DeleteTargets atomically releases the specified allocations.
	DeleteTargets(context.Context, []Target) error
	// DeleteWorkspace releases one installation across its profile projections atomically.
	DeleteWorkspace(context.Context, string, string) error
	// RetireWorkspace atomically releases allocations and records the completed lifecycle event.
	RetireWorkspace(context.Context, string, string, store.EventSummary) error
	DeleteInstance(context.Context, string, string, string) error
}
