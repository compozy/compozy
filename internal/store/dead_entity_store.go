package store

import "context"

// DeadEntityStore manages workspace-scoped confirmed-dead external runtimes.
type DeadEntityStore interface {
	MarkDeadEntity(ctx context.Context, entity DeadEntity) error
	ClearDeadEntity(ctx context.Context, workspaceID string, kind DeadEntityKind, entityID string) error
	FindDeadEntity(
		ctx context.Context,
		workspaceID string,
		kind DeadEntityKind,
		entityID string,
	) (DeadEntity, bool, error)
	ListDeadEntities(ctx context.Context, workspaceID string) ([]DeadEntity, error)
}
