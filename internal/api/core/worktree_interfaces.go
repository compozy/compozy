package core

import (
	"context"

	"github.com/compozy/compozy/internal/worktree"
)

type WorktreeService interface {
	CreateAccepted(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	CreateReady(context.Context, string, worktree.CreateOptions) (*worktree.Worktree, error)
	CancelCreate(context.Context, string, string) error
	Adopt(context.Context, string, string) (*worktree.Worktree, error)
	ListDetails(context.Context, string, bool) (*worktree.DetailedListing, error)
	Inspect(context.Context, string, string) (*worktree.Inspection, error)
	StatusDetails(context.Context, string, string, bool, bool) (*worktree.StatusDetails, error)
	Remove(context.Context, string, string, bool) (*worktree.RemovalRefusal, error)
	Dismiss(context.Context, string, string) error
}

type WorktreeCatalogEventSubscriber interface {
	SubscribeWorktreeCatalogEvents(context.Context) (<-chan worktree.CatalogEvent, func(), error)
}
