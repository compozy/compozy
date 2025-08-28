package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors for consistent error handling
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrAPIKeyNotFound = errors.New("API key not found")
)

// AuthRepo implements the auth.Repository interface
type AuthRepo struct {
	db *pgxpool.Pool
}

// NewAuthRepo creates a new AuthRepo
func NewAuthRepo(db *pgxpool.Pool) *AuthRepo {
	return &AuthRepo{db: db}
}

// Ensure AuthRepo implements the uc.Repository interface
var _ uc.Repository = (*AuthRepo)(nil)

// CreateUser creates a new user in the database
func (r *AuthRepo) CreateUser(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (id, email, role, created_at)
		VALUES ($1, $2, $3, $4)
	`
	now := time.Now()
	user.CreatedAt = now
	_, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (r *AuthRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	query := `
		SELECT id, email, role, created_at
		FROM users
		WHERE id = $1
	`
	var user model.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, role, created_at
		FROM users
		WHERE email = $1
	`
	var user model.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// ListUsers lists all users
func (r *AuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	query := `
		SELECT id, email, role, created_at
		FROM users
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()
	var users []*model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}
	return users, nil
}

// UpdateUser updates a user
func (r *AuthRepo) UpdateUser(ctx context.Context, user *model.User) error {
	query := `
		UPDATE users
		SET email = $2, role = $3, updated_at = $4
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// DeleteUser deletes a user
func (r *AuthRepo) DeleteUser(ctx context.Context, id core.ID) error {
	// First delete all API keys for the user
	_, err := r.db.Exec(ctx, "DELETE FROM api_keys WHERE user_id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user API keys: %w", err)
	}
	// Then delete the user
	result, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// CreateAPIKey creates a new API key
func (r *AuthRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	query := `
		INSERT INTO api_keys (id, user_id, hash, prefix, fingerprint, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	createdAt := time.Now()
	_, err := r.db.Exec(ctx, query, key.ID, key.UserID, key.Hash, key.Prefix, key.Fingerprint, createdAt)
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}
	return nil
}

// GetAPIKeyByID retrieves an API key by ID
func (r *AuthRepo) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	query := `
		SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used
		FROM api_keys
		WHERE id = $1
	`
	var key model.APIKey
	err := r.db.QueryRow(ctx, query, id).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Prefix,
		&key.Fingerprint,
		&key.CreatedAt,
		&key.LastUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

// GetAPIKeyByHash retrieves an API key by its fingerprint hash (SHA256)
func (r *AuthRepo) GetAPIKeyByHash(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	// Use the provided fingerprint directly for lookup
	query := `
		SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used
		FROM api_keys
		WHERE fingerprint = $1
	`
	var key model.APIKey
	err := r.db.QueryRow(ctx, query, fingerprint).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Prefix,
		&key.Fingerprint,
		&key.CreatedAt,
		&key.LastUsed,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

// ListAPIKeysByUserID lists all API keys for a user
func (r *AuthRepo) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	query := `
		SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()
	var keys []*model.APIKey
	for rows.Next() {
		var key model.APIKey
		err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Hash,
			&key.Prefix,
			&key.Fingerprint,
			&key.CreatedAt,
			&key.LastUsed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, &key)
	}
	return keys, nil
}

// UpdateAPIKeyLastUsed updates the last used timestamp for an API key
func (r *AuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	query := `
		UPDATE api_keys
		SET last_used = $2
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, sql.NullTime{Time: time.Now(), Valid: true})
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

// DeleteAPIKey deletes an API key
func (r *AuthRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM api_keys WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

// CreateInitialAdminIfNone creates the initial admin user if no admin exists
func (r *AuthRepo) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	// Check if any admin user exists
	var adminExists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE role = $1)", model.RoleAdmin).Scan(&adminExists)
	if err != nil {
		return fmt.Errorf("failed to check for existing admin: %w", err)
	}
	// If an admin already exists, return an error
	if adminExists {
		return core.NewError(
			fmt.Errorf("system already bootstrapped"),
			"ALREADY_BOOTSTRAPPED",
			nil,
		)
	}
	// Create the initial admin user
	return r.CreateUser(ctx, user)
}
