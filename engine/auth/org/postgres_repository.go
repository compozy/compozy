package org

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresRepository implements Repository using PostgreSQL
type postgresRepository struct {
	db store.DBInterface
}

// NewPostgresRepository creates a new PostgreSQL repository instance
func NewPostgresRepository(db store.DBInterface) Repository {
	return &postgresRepository{db: db}
}

// scanOrganization is a helper function to scan a database row into an Organization struct
func scanOrganization(scannable interface{ Scan(dest ...any) error }) (*Organization, error) {
	var org Organization
	err := scannable.Scan(
		&org.ID,
		&org.Name,
		&org.TemporalNamespace,
		&org.Status,
		&org.CreatedAt,
		&org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return &org, nil
}

// Create creates a new organization
func (r *postgresRepository) Create(ctx context.Context, org *Organization) error {
	query := `INSERT INTO organizations (id, name, temporal_namespace, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query,
		org.ID,
		org.Name,
		org.TemporalNamespace,
		org.Status,
		org.CreatedAt,
		org.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}
	return nil
}

// GetByID retrieves an organization by its ID
func (r *postgresRepository) GetByID(ctx context.Context, id core.ID) (*Organization, error) {
	query := `SELECT id, name, temporal_namespace, status, created_at, updated_at
FROM organizations
WHERE id = $1`
	org, err := scanOrganization(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get organization by ID: %w", err)
	}
	return org, nil
}

// GetByName retrieves an organization by its name
func (r *postgresRepository) GetByName(ctx context.Context, name string) (*Organization, error) {
	query := `SELECT id, name, temporal_namespace, status, created_at, updated_at
FROM organizations
WHERE name = $1`
	org, err := scanOrganization(r.db.QueryRow(ctx, query, name))
	if err != nil {
		if errors.Is(err, ErrOrganizationNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get organization by name: %w", err)
	}
	return org, nil
}

// Update updates an existing organization
func (r *postgresRepository) Update(ctx context.Context, org *Organization) error {
	query := `UPDATE organizations
SET name = $2, temporal_namespace = $3, status = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1`
	result, err := r.db.Exec(ctx, query,
		org.ID,
		org.Name,
		org.TemporalNamespace,
		org.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// Delete deletes an organization by its ID
func (r *postgresRepository) Delete(ctx context.Context, id core.ID) error {
	query := `DELETE FROM organizations WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// List retrieves organizations with pagination
func (r *postgresRepository) List(ctx context.Context, limit, offset int) ([]*Organization, error) {
	query := `SELECT id, name, temporal_namespace, status, created_at, updated_at
FROM organizations
ORDER BY created_at DESC
LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()
	var orgs []*Organization
	for rows.Next() {
		org, err := scanOrganization(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating organization rows: %w", err)
	}
	return orgs, nil
}

// UpdateStatus updates the status of an organization
func (r *postgresRepository) UpdateStatus(ctx context.Context, id core.ID, status OrganizationStatus) error {
	query := `UPDATE organizations
SET status = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update organization status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// FindByName searches for organizations by name pattern
func (r *postgresRepository) FindByName(ctx context.Context, namePattern string) ([]*Organization, error) {
	query := `SELECT id, name, temporal_namespace, status, created_at, updated_at
FROM organizations
WHERE name ILIKE $1
ORDER BY name`
	rows, err := r.db.Query(ctx, query, "%"+namePattern+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to find organizations by name: %w", err)
	}
	defer rows.Close()
	var orgs []*Organization
	for rows.Next() {
		org, err := scanOrganization(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating organization rows: %w", err)
	}
	return orgs, nil
}

// WithTx returns a repository instance that uses the given transaction.
// The pgx.Tx type satisfies the store.DBInterface, allowing seamless transaction support.
func (r *postgresRepository) WithTx(tx pgx.Tx) Repository {
	return &postgresRepository{db: tx}
}

// Compile-time checks
var (
	_ Repository = (*postgresRepository)(nil)
	// Ensure pgx.Tx satisfies store.DBInterface
	_ store.DBInterface = (pgx.Tx)(nil)
)

// Helper function to create repository from pool
func NewPostgresRepositoryFromPool(pool *pgxpool.Pool) Repository {
	return NewPostgresRepository(pool)
}
