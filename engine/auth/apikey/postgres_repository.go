package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// postgresRepository implements Repository using PostgreSQL
type postgresRepository struct {
	db store.DBInterface
}

// NewPostgresRepository creates a new PostgreSQL repository instance
func NewPostgresRepository(db store.DBInterface) Repository {
	return &postgresRepository{db: db}
}

// scanAPIKey is a helper function to scan a database row into an APIKey struct
func scanAPIKey(scannable interface{ Scan(dest ...any) error }) (*APIKey, error) {
	var apiKey APIKey
	err := scannable.Scan(
		&apiKey.ID,
		&apiKey.OrgID,
		&apiKey.UserID,
		&apiKey.KeyPrefix,
		&apiKey.KeyHash,
		&apiKey.Name,
		&apiKey.Status,
		&apiKey.ExpiresAt,
		&apiKey.LastUsedAt,
		&apiKey.CreatedAt,
		&apiKey.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return &apiKey, nil
}

// Create creates a new API key
func (r *postgresRepository) Create(ctx context.Context, apiKey *APIKey) error {
	query := `
		INSERT INTO api_keys (id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		apiKey.ID,
		apiKey.OrgID,
		apiKey.UserID,
		apiKey.KeyPrefix,
		apiKey.KeyHash,
		apiKey.Name,
		apiKey.Status,
		apiKey.ExpiresAt,
		apiKey.CreatedAt,
		apiKey.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}
	return nil
}

// GetByID retrieves an API key by its ID within an organization
func (r *postgresRepository) GetByID(ctx context.Context, orgID, keyID core.ID) (*APIKey, error) {
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1 AND id = $2
	`
	apiKey, err := scanAPIKey(r.db.QueryRow(ctx, query, orgID, keyID))
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get API key by ID: %w", err)
	}
	return apiKey, nil
}

// GetByPrefix retrieves an API key by its key_prefix within an organization
func (r *postgresRepository) GetByPrefix(ctx context.Context, orgID core.ID, prefix string) (*APIKey, error) {
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1 AND key_prefix = $2
	`
	apiKey, err := scanAPIKey(r.db.QueryRow(ctx, query, orgID, prefix))
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get API key by key_prefix: %w", err)
	}
	return apiKey, nil
}

// Update updates an existing API key
func (r *postgresRepository) Update(ctx context.Context, apiKey *APIKey) error {
	query := `
		UPDATE api_keys
		SET name = $3, status = $4, expires_at = $5, last_used_at = $6, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query,
		apiKey.OrgID,
		apiKey.ID,
		apiKey.Name,
		apiKey.Status,
		apiKey.ExpiresAt,
		apiKey.LastUsedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// Delete deletes an API key by its ID within an organization
func (r *postgresRepository) Delete(ctx context.Context, orgID, keyID core.ID) error {
	query := `DELETE FROM api_keys WHERE org_id = $1 AND id = $2`
	result, err := r.db.Exec(ctx, query, orgID, keyID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// List retrieves API keys within an organization with pagination
func (r *postgresRepository) List(ctx context.Context, orgID core.ID, limit, offset int) ([]*APIKey, error) {
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()
	var apiKeys []*APIKey
	for rows.Next() {
		apiKey, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}
	return apiKeys, nil
}

// ListByUser retrieves API keys for a specific user within an organization
func (r *postgresRepository) ListByUser(
	ctx context.Context,
	orgID, userID core.ID,
	limit, offset int,
) ([]*APIKey, error) {
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, orgID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys by user: %w", err)
	}
	defer rows.Close()
	var apiKeys []*APIKey
	for rows.Next() {
		apiKey, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}
	return apiKeys, nil
}

// ListActive retrieves active API keys within an organization
func (r *postgresRepository) ListActive(ctx context.Context, orgID core.ID, limit, offset int) ([]*APIKey, error) {
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1 AND status = $2 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, orgID, StatusActive, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list active API keys: %w", err)
	}
	defer rows.Close()
	var apiKeys []*APIKey
	for rows.Next() {
		apiKey, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}
	return apiKeys, nil
}

// ValidateKey validates an API key by checking its hash and status
func (r *postgresRepository) ValidateKey(ctx context.Context, orgID core.ID, plainTextKey string) (*APIKey, error) {
	// Extract prefix from the plain text key
	prefix, err := ExtractPrefix(plainTextKey)
	if err != nil {
		// Return generic error to prevent information leakage
		return nil, ErrInvalidAPIKey
	}
	// Get the API key by prefix
	apiKey, err := r.GetByPrefix(ctx, orgID, prefix)
	if err != nil {
		// Return generic error to prevent information leakage about key existence
		return nil, ErrInvalidAPIKey
	}
	// Validate bcrypt cost to prevent DoS attacks
	cost, err := bcrypt.Cost([]byte(apiKey.KeyHash))
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	if cost < 10 || cost > 12 {
		// Log potential security issue
		log := logger.FromContext(ctx)
		log.With("api_key_id", apiKey.ID, "cost", cost).
			Error("API key hash cost outside allowed range (10-12)")
		return nil, ErrInvalidAPIKey
	}
	// Check if the key is active
	if apiKey.Status != StatusActive {
		// Return generic error to prevent status enumeration
		return nil, ErrInvalidAPIKey
	}
	// Check if the key has expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now().UTC()) {
		// Return generic error to prevent expiration status leakage
		return nil, ErrInvalidAPIKey
	}
	// Verify the key hash
	err = bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(plainTextKey))
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	// Update last used timestamp atomically with validation
	if err := r.ValidateAndUpdateLastUsed(ctx, orgID, apiKey.ID); err != nil {
		// If the atomic update fails, the key might have been revoked/expired
		// between our initial check and the update - return error to prevent race condition
		log := logger.FromContext(ctx)
		log.With("api_key_id", apiKey.ID, "error", err).
			Warn("API key validation failed during atomic update")
		return nil, ErrInvalidAPIKey
	}
	return apiKey, nil
}

// UpdateStatus updates the status of an API key
func (r *postgresRepository) UpdateStatus(ctx context.Context, orgID, keyID core.ID, status Status) error {
	query := `
		UPDATE api_keys
		SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query, orgID, keyID, status)
	if err != nil {
		return fmt.Errorf("failed to update API key status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// UpdateLastUsed updates the last used timestamp of an API key
func (r *postgresRepository) UpdateLastUsed(ctx context.Context, orgID, keyID core.ID, lastUsed time.Time) error {
	query := `
		UPDATE api_keys
		SET last_used_at = $3, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query, orgID, keyID, lastUsed)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// ValidateAndUpdateLastUsed atomically validates and updates the last used timestamp
func (r *postgresRepository) ValidateAndUpdateLastUsed(ctx context.Context, orgID, keyID core.ID) error {
	query := `
		UPDATE api_keys
		SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
		AND status = $3
		AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(ctx, query, orgID, keyID, StatusActive)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	if result.RowsAffected() == 0 {
		// Key might have been deactivated or expired between validation and update
		return ErrAPIKeyNotFound
	}
	return nil
}

// RevokeExpired revokes all expired API keys within an organization
func (r *postgresRepository) RevokeExpired(ctx context.Context, orgID core.ID) (int64, error) {
	query := `
		UPDATE api_keys
		SET status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND status = $3 AND expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP
	`
	result, err := r.db.Exec(ctx, query, orgID, StatusExpired, StatusActive)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke expired API keys: %w", err)
	}
	return result.RowsAffected(), nil
}

// CountByOrg returns the total count of API keys in an organization
func (r *postgresRepository) CountByOrg(ctx context.Context, orgID core.ID) (int64, error) {
	query := `SELECT COUNT(*) FROM api_keys WHERE org_id = $1`
	var count int64
	err := r.db.QueryRow(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}
	return count, nil
}

// CountByUser returns the total count of API keys for a user
func (r *postgresRepository) CountByUser(ctx context.Context, orgID, userID core.ID) (int64, error) {
	query := `SELECT COUNT(*) FROM api_keys WHERE org_id = $1 AND user_id = $2`
	var count int64
	err := r.db.QueryRow(ctx, query, orgID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys by user: %w", err)
	}
	return count, nil
}

// FindByPrefix searches for API keys by key_prefix pattern within an organization
func (r *postgresRepository) FindByPrefix(
	ctx context.Context,
	orgID core.ID,
	prefixPattern string,
) ([]*APIKey, error) {
	// Require minimum pattern length to prevent DOS with short patterns
	if len(prefixPattern) < 4 {
		return nil, errors.New("search pattern must be at least 4 characters")
	}
	query := `
		SELECT id, org_id, user_id, key_prefix, key_hash, name, status, expires_at, last_used_at, created_at, updated_at
		FROM api_keys
		WHERE org_id = $1 AND key_prefix ILIKE $2
		ORDER BY key_prefix
	`
	rows, err := r.db.Query(ctx, query, orgID, "%"+prefixPattern+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to find API keys by key_prefix: %w", err)
	}
	defer rows.Close()
	var apiKeys []*APIKey
	for rows.Next() {
		apiKey, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		apiKeys = append(apiKeys, apiKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API key rows: %w", err)
	}
	return apiKeys, nil
}

// WithTx returns a repository instance that uses the given transaction
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
