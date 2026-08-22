package store

import "context"

// DeadEntityStore manages workspace-scoped confirmed-dead external runtimes.
type DeadEntityStore interface {
	MarkDeadEntity(ctx context.Context, entity DeadEntity) error
	ClearDeadEntity(ctx context.Context, key DeadEntityKey) error
	FindDeadEntity(
		ctx context.Context,
		key DeadEntityKey,
	) (DeadEntity, bool, error)
	ListDeadEntities(ctx context.Context, readScope ReadScope, workspaceID string) ([]DeadEntity, error)
}
