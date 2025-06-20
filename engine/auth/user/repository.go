package user

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for user data access
type Repository interface {
	// Create creates a new user
	Create(ctx context.Context, user *User) error
	// GetByID retrieves a user by its ID within an organization
	GetByID(ctx context.Context, orgID, userID core.ID) (*User, error)
	// GetByEmail retrieves a user by email within an organization
	GetByEmail(ctx context.Context, orgID core.ID, email string) (*User, error)
	// Update updates an existing user
	Update(ctx context.Context, user *User) error
	// Delete deletes a user by its ID within an organization
	Delete(ctx context.Context, orgID, userID core.ID) error
	// List retrieves users within an organization with pagination
	List(ctx context.Context, orgID core.ID, limit, offset int) ([]*User, error)
	// ListByRole retrieves users by role within an organization
	ListByRole(ctx context.Context, orgID core.ID, role string, limit, offset int) ([]*User, error)
	// UpdateRole updates the role of a user
	UpdateRole(ctx context.Context, orgID, userID core.ID, role string) error
	// UpdateStatus updates the status of a user
	UpdateStatus(ctx context.Context, orgID, userID core.ID, status Status) error
	// FindByEmail searches for users by email pattern within an organization
	FindByEmail(ctx context.Context, orgID core.ID, emailPattern string) ([]*User, error)
	// CountByOrg returns the total count of users in an organization
	CountByOrg(ctx context.Context, orgID core.ID) (int64, error)
	// WithTx returns a repository instance that uses the given transaction
	WithTx(tx pgx.Tx) Repository
}
