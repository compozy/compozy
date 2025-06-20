package org

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for organization data access
type Repository interface {
	// Create creates a new organization
	Create(ctx context.Context, org *Organization) error
	// GetByID retrieves an organization by its ID
	GetByID(ctx context.Context, id core.ID) (*Organization, error)
	// GetByName retrieves an organization by its name
	GetByName(ctx context.Context, name string) (*Organization, error)
	// Update updates an existing organization
	Update(ctx context.Context, org *Organization) error
	// Delete deletes an organization by its ID
	Delete(ctx context.Context, id core.ID) error
	// List retrieves organizations with pagination
	List(ctx context.Context, limit, offset int) ([]*Organization, error)
	// UpdateStatus updates the status of an organization
	UpdateStatus(ctx context.Context, id core.ID, status OrganizationStatus) error
	// FindByName searches for organizations by name pattern
	FindByName(ctx context.Context, namePattern string) ([]*Organization, error)
	// WithTx returns a repository instance that uses the given transaction
	WithTx(tx pgx.Tx) Repository
}
