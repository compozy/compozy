package repository

import "context"

// GithubRepository defines the interface for GitHub API operations.

type GithubRepository interface {
	CreatePullRequest(ctx context.Context, title, body, head, base string) (int, error)
}
