package apikey

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for API key data access
type Repository interface {
	// Create creates a new API key
	Create(ctx context.Context, apiKey *APIKey) error
	// GetByID retrieves an API key by its ID within an organization
	GetByID(ctx context.Context, orgID, keyID core.ID) (*APIKey, error)
	// GetByPrefix retrieves an API key by its prefix within an organization
	GetByPrefix(ctx context.Context, orgID core.ID, prefix string) (*APIKey, error)
	// Update updates an existing API key
	Update(ctx context.Context, apiKey *APIKey) error
	// Delete deletes an API key by its ID within an organization
	Delete(ctx context.Context, orgID, keyID core.ID) error
	// List retrieves API keys within an organization with pagination
	List(ctx context.Context, orgID core.ID, limit, offset int) ([]*APIKey, error)
	// ListByUser retrieves API keys for a specific user within an organization
	ListByUser(ctx context.Context, orgID, userID core.ID, limit, offset int) ([]*APIKey, error)
	// ListActive retrieves active API keys within an organization
	ListActive(ctx context.Context, orgID core.ID, limit, offset int) ([]*APIKey, error)
	// UpdateStatus updates the status of an API key
	UpdateStatus(ctx context.Context, orgID, keyID core.ID, status Status) error
	// UpdateLastUsed updates the last used timestamp of an API key
	UpdateLastUsed(ctx context.Context, orgID, keyID core.ID, lastUsed time.Time) error
	// ValidateAndUpdateLastUsed atomically validates and updates the last used timestamp
	ValidateAndUpdateLastUsed(ctx context.Context, orgID, keyID core.ID) error
	// RevokeExpired revokes all expired API keys within an organization
	RevokeExpired(ctx context.Context, orgID core.ID) (int64, error)
	// CountByOrg returns the total count of API keys in an organization
	CountByOrg(ctx context.Context, orgID core.ID) (int64, error)
	// CountByUser returns the total count of API keys for a user
	CountByUser(ctx context.Context, orgID, userID core.ID) (int64, error)
	// FindByPrefix searches for API keys by prefix pattern within an organization
	FindByPrefix(ctx context.Context, orgID core.ID, prefixPattern string) ([]*APIKey, error)
	// FindByExactPrefix finds a single API key by its exact prefix across all organizations
	// This is used for API key validation where we don't know the organization yet
	FindByExactPrefix(ctx context.Context, prefix string) (*APIKey, error)
	// WithTx returns a repository instance that uses the given transaction
	WithTx(tx pgx.Tx) Repository
}
