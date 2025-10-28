package uc

import (
	"context"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
)

// Repository defines all data access operations for the auth domain
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByID(ctx context.Context, id core.ID) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	ListUsers(ctx context.Context) ([]*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	DeleteUser(ctx context.Context, id core.ID) error

	// CreateInitialAdminIfNone atomically creates the initial admin user if no admin exists.
	// Returns ErrAlreadyBootstrapped if an admin user already exists.
	CreateInitialAdminIfNone(ctx context.Context, user *model.User) error

	// API Key operations
	CreateAPIKey(ctx context.Context, key *model.APIKey) error
	GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error)
	GetAPIKeyByFingerprint(ctx context.Context, fingerprint []byte) (*model.APIKey, error)
	ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error
	DeleteAPIKey(ctx context.Context, id core.ID) error
}
