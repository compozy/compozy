package repository

import "context"

// GitExtendedRepository extends GitRepository with additional operations needed for orchestration.
type GitExtendedRepository interface {
	GitRepository
	// Checkout operations
	CheckoutBranch(ctx context.Context, name string) error
	// Git configuration
	ConfigureUser(ctx context.Context, name, email string) error
	// Staging operations
	AddFiles(ctx context.Context, pattern string) error
	// Commit operations
	Commit(ctx context.Context, message string) error
	GetHeadCommit(ctx context.Context) (string, error)
	// Branch operations
	GetCurrentBranch(ctx context.Context) (string, error)
	PushBranch(ctx context.Context, branch string) error
	PushBranchForce(ctx context.Context, branch string) error
	DeleteBranch(ctx context.Context, name string) error
	DeleteRemoteBranch(ctx context.Context, name string) error
	ListLocalBranches(ctx context.Context) ([]string, error)
	ListRemoteBranches(ctx context.Context) ([]string, error)
	// Tag operations
	TagExists(ctx context.Context, tag string) (bool, error)
	// File operations
	RestoreFile(ctx context.Context, path string) error
	ResetHard(ctx context.Context, ref string) error
	GetFileStatus(ctx context.Context, path string) (string, error)
}
