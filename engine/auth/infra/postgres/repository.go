package postgres

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// Repository implements the auth repository interface using PostgreSQL
type Repository struct {
	db DBInterface
}

// DBInterface defines the minimal interface needed by the repository
type DBInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// NewRepository creates a new auth repository
func NewRepository(db DBInterface) uc.Repository {
	return &Repository{db: db}
}

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, user *model.User) error {
	query, args, err := squirrel.Insert("users").
		Columns("id", "email", "role", "created_at").
		Values(user.ID, user.Email, user.Role, user.CreatedAt).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("building insert query: %w", err)
	}
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	query, args, err := squirrel.Select("id", "email", "role", "created_at").
		From("users").
		Where(squirrel.Eq{"id": id}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}
	var user model.User
	if err := pgxscan.Get(ctx, r.db, &user, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email (case-insensitive)
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	query, args, err := squirrel.Select("id", "email", "role", "created_at").
		From("users").
		Where("lower(email) = lower(?)", email).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}
	var user model.User
	if err := pgxscan.Get(ctx, r.db, &user, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	return &user, nil
}

// ListUsers retrieves all users
func (r *Repository) ListUsers(ctx context.Context) ([]*model.User, error) {
	qb := squirrel.Select("id", "email", "role", "created_at").
		From("users").
		OrderBy("created_at DESC").
		PlaceholderFormat(squirrel.Dollar)

	query, args, err := qb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}
	var users []*model.User
	if err := pgxscan.Select(ctx, r.db, &users, query, args...); err != nil {
		return nil, fmt.Errorf("scanning users: %w", err)
	}
	return users, nil
}

// UpdateUser updates user fields
func (r *Repository) UpdateUser(ctx context.Context, user *model.User) error {
	query, args, err := squirrel.Update("users").
		Set("email", user.Email).
		Set("role", user.Role).
		Where(squirrel.Eq{"id": user.ID}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("building update query: %w", err)
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// DeleteUser removes a user by ID
func (r *Repository) DeleteUser(ctx context.Context, id core.ID) error {
	query, args, err := squirrel.Delete("users").
		Where(squirrel.Eq{"id": id}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("building delete query: %w", err)
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// CreateAPIKey creates a new API key
func (r *Repository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	query, args, err := squirrel.Insert("api_keys").
		Columns("id", "user_id", "hash", "fingerprint", "prefix", "created_at", "last_used").
		Values(key.ID, key.UserID, key.Hash, key.Fingerprint, key.Prefix, key.CreatedAt, key.LastUsed).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("building insert query: %w", err)
	}
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("inserting API key: %w", err)
	}
	return nil
}

// GetAPIKeyByID retrieves an API key by ID
func (r *Repository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	query, args, err := squirrel.Select("id", "user_id", "hash", "fingerprint", "prefix", "created_at", "last_used").
		From("api_keys").
		Where(squirrel.Eq{"id": id}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}
	var key model.APIKey
	if err := pgxscan.Get(ctx, r.db, &key, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("scanning API key: %w", err)
	}
	return &key, nil
}

// GetAPIKeyByHash retrieves an API key by its hash for validation.
//
// SECURITY NOTE: This function implements timing attack prevention by always performing
// bcrypt operations even when the key is not found. While this is secure, it creates
// a potential DoS vector as bcrypt is computationally expensive. Ensure aggressive
// IP-based rate limiting is applied in middleware BEFORE this authentication check.
func (r *Repository) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	log := logger.FromContext(ctx)

	// Generate SHA-256 fingerprint for O(1) lookup
	fingerprintHash := sha256.Sum256(hash)
	fingerprint := fingerprintHash[:]

	// Query by fingerprint for O(1) performance instead of O(n)
	query, args, err := squirrel.Select("id", "user_id", "hash", "fingerprint", "prefix", "created_at", "last_used").
		From("api_keys").
		Where(squirrel.Eq{"fingerprint": fingerprint}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}

	var key model.APIKey
	var dbErr error
	if err := pgxscan.Get(ctx, r.db, &key, query, args...); err != nil {
		if pgxscan.NotFound(err) {
			// To prevent timing attacks, always perform bcrypt comparison even when key not found
			// Use a valid dummy bcrypt hash with same computational cost as a real bcrypt hash
			// This is a valid bcrypt hash for "dummyPassword" with cost 10
			dummyHash := []byte("$2a$10$g.L2nI52OAiN/O8Qk25SluXfK090sjsV2e9.j2y.Xy.Z2.a4.b6cK")
			// Dummy operation - error is expected and ignored for timing attack prevention
			_ = bcrypt.CompareHashAndPassword( //nolint:errcheck // intentional dummy operation for timing attack prevention
				dummyHash,
				hash,
			)
			return nil, fmt.Errorf("API key not found")
		}
		dbErr = err
	}

	// Always perform bcrypt comparison to ensure constant time operation
	var bcryptErr error
	if dbErr == nil {
		bcryptErr = bcrypt.CompareHashAndPassword(key.Hash, hash)
	} else {
		// Database error case - still perform dummy bcrypt to maintain constant time
		// Use the same valid dummy bcrypt hash to ensure consistent timing
		dummyHash := []byte("$2a$10$g.L2nI52OAiN/O8Qk25SluXfK090sjsV2e9.j2y.Xy.Z2.a4.b6cK")
		// Dummy operation - error is expected and ignored for timing attack prevention
		_ = bcrypt.CompareHashAndPassword( //nolint:errcheck // intentional dummy operation for timing attack prevention
			dummyHash,
			hash,
		)
		return nil, fmt.Errorf("scanning API key: %w", dbErr)
	}

	// Check bcrypt result
	if bcryptErr != nil {
		log.Debug("API key validation failed")
		return nil, fmt.Errorf("API key not found")
	}

	log.Debug("API key validated successfully", "key_id", key.ID)
	return &key, nil
}

// ListAPIKeysByUserID retrieves all API keys for a user
func (r *Repository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	query, args, err := squirrel.Select("id", "user_id", "hash", "fingerprint", "prefix", "created_at", "last_used").
		From("api_keys").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("building select query: %w", err)
	}
	var keys []*model.APIKey
	if err := pgxscan.Select(ctx, r.db, &keys, query, args...); err != nil {
		return nil, fmt.Errorf("scanning API keys: %w", err)
	}
	return keys, nil
}

// UpdateAPIKeyLastUsed updates the last_used timestamp for an API key
func (r *Repository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	// Use GREATEST to prevent race condition overwrites as documented in migration
	query := `UPDATE api_keys SET last_used = GREATEST(last_used, NOW()) WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("updating API key last_used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}
	return nil
}

// DeleteAPIKey removes an API key by ID
func (r *Repository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	query, args, err := squirrel.Delete("api_keys").
		Where(squirrel.Eq{"id": id}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("building delete query: %w", err)
	}
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting API key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("API key not found")
	}
	return nil
}

// CreateInitialAdminIfNone atomically creates the initial admin user if no admin exists.
// Uses atomic INSERT...WHERE NOT EXISTS to prevent race conditions.
// Returns ErrAlreadyBootstrapped if an admin user already exists.
func (r *Repository) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	// Use atomic INSERT...WHERE NOT EXISTS to prevent race conditions
	// This eliminates the check-then-act pattern that could allow concurrent duplicates
	query := `
		INSERT INTO users (id, email, role, created_at)
		SELECT $1, $2, $3, $4
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE role = $5)
	`
	tag, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, user.CreatedAt, model.RoleAdmin)
	if err != nil {
		return fmt.Errorf("creating initial admin user: %w", err)
	}

	// If no rows were affected, an admin already exists
	if tag.RowsAffected() == 0 {
		return core.NewError(
			fmt.Errorf("system already bootstrapped"),
			"ALREADY_BOOTSTRAPPED",
			nil,
		)
	}
	return nil
}
