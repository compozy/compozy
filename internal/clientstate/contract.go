// Package clientstate persists opaque, workspace-scoped client state and
// publishes committed changes in workspace sequence order.
package clientstate

import (
	"context"
	"time"
)

// WorkspaceID identifies the workspace that owns a state partition.
type WorkspaceID string

// WorkspaceGeneration uniquely identifies one registration of a workspace ID.
type WorkspaceGeneration string

// WorkspaceResolver gates access to registered workspaces.
//
// ResolveWorkspace rejects unknown or deleting workspaces. Purge resolution
// succeeds only while the matching registration is held behind its deletion
// gate. Implementations must never reuse a generation for a recreated ID.
type WorkspaceResolver interface {
	ResolveWorkspace(ctx context.Context, ws WorkspaceID) (WorkspaceGeneration, error)
	ResolveWorkspaceForPurge(ctx context.Context, ws WorkspaceID) (WorkspaceGeneration, error)
}

// Service is the client-state runtime contract.
type Service interface {
	Get(ctx context.Context, ws WorkspaceID, domain, key string) (Entry, error)
	List(ctx context.Context, ws WorkspaceID, domain string) ([]Entry, error)
	Apply(
		ctx context.Context,
		ws WorkspaceID,
		domain string,
		ops []Op,
		opts ApplyOptions,
	) ([]Entry, error)
	Watch(ctx context.Context, ws WorkspaceID, domains []string) (Subscription, error)
	PurgeWorkspace(ctx context.Context, ws WorkspaceID) error
}

// WorkspacePurgePreparation is a reversible physical purge staged before the
// owning workspace row is deleted.
type WorkspacePurgePreparation interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// OpKind identifies one mutation in an atomic Apply batch.
type OpKind uint8

const (
	// OpPut creates or replaces a live value.
	OpPut OpKind = iota + 1
	// OpDelete writes a tombstone for an existing live value.
	OpDelete
)

// Op is one mutation in an Apply batch.
type Op struct {
	Kind  OpKind
	Key   string
	Value []byte
	IfRev uint64
}

// ApplyOptions carries mutation metadata that is not persisted in the value.
type ApplyOptions struct {
	Origin string
}

// Subscription composes a snapshot fence with later committed events.
type Subscription interface {
	AsOfSeq() uint64
	Snapshot() []Entry
	Events() <-chan Event
	Err() error
	Close() error
}

// Entry is the durable envelope for one client-state value.
type Entry struct {
	Domain    string
	Key       string
	Value     []byte
	Rev       uint64
	Seq       uint64
	Deleted   bool
	UpdatedAt time.Time
}

// Event is one committed entry published to workspace subscribers.
type Event struct {
	Workspace WorkspaceID
	Entry     Entry
	Origin    string
}

var _ Service = (*Engine)(nil)
