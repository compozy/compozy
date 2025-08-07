package repository

import "context"

// GitRepository defines the interface for Git operations.

type GitRepository interface {
	LatestTag(ctx context.Context) (string, error)
	CommitsSinceTag(ctx context.Context, tag string) (int, error)
	TagExists(ctx context.Context, tag string) (bool, error)
	CreateBranch(ctx context.Context, name string) error
	CreateTag(ctx context.Context, tag, msg string) error
	PushTag(ctx context.Context, tag string) error
	PushBranch(ctx context.Context, name string) error
}
