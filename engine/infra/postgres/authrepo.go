package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuthRepo implements uc.Repository using pgxpool.
type AuthRepo struct{ db *pgxpool.Pool }

func NewAuthRepo(db *pgxpool.Pool) uc.Repository { return &AuthRepo{db: db} }

func (r *AuthRepo) CreateUser(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (id, email, role, created_at) VALUES ($1, $2, $3, $4)`
	user.CreatedAt = time.Now()
	if _, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, user.CreatedAt); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *AuthRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	query := `SELECT id, email, role, created_at FROM users WHERE id = $1`
	var user model.User
	if err := r.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, uc.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, email, role, created_at FROM users WHERE email = $1`
	var user model.User
	if err := r.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, uc.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *AuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	query := `SELECT id, email, role, created_at FROM users ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()
	var users []*model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	return users, nil
}

func (r *AuthRepo) UpdateUser(ctx context.Context, user *model.User) error {
	query := `UPDATE users SET email = $2, role = $3, updated_at = $4 WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return uc.ErrUserNotFound
	}
	return nil
}

func (r *AuthRepo) DeleteUser(ctx context.Context, id core.ID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				logger.FromContext(ctx).Warn("Transaction rollback failed", "error", rbErr)
			}
		}
	}()
	if _, err = tx.Exec(ctx, "DELETE FROM api_keys WHERE user_id = $1", id); err != nil {
		return fmt.Errorf("failed to delete user API keys: %w", err)
	}
	var tag pgconn.CommandTag
	tag, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return uc.ErrUserNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *AuthRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	query := `INSERT INTO api_keys (id, user_id, hash, prefix, fingerprint, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	createdAt := time.Now()
	if _, err := r.db.Exec(
		ctx,
		query,
		key.ID,
		key.UserID,
		key.Hash,
		key.Prefix,
		key.Fingerprint,
		createdAt,
	); err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}
	return nil
}

func (r *AuthRepo) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	query := `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE id = $1`
	var key model.APIKey
	if err := r.db.QueryRow(ctx, query, id).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Prefix,
		&key.Fingerprint,
		&key.CreatedAt,
		&key.LastUsed,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, uc.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

func (r *AuthRepo) GetAPIKeyByFingerprint(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	query := `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE fingerprint = $1`
	var key model.APIKey
	if err := r.db.QueryRow(ctx, query, fingerprint).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Prefix,
		&key.Fingerprint,
		&key.CreatedAt,
		&key.LastUsed,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, uc.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}
	return &key, nil
}

func (r *AuthRepo) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	query := `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()
	var keys []*model.APIKey
	for rows.Next() {
		var key model.APIKey
		if err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Hash,
			&key.Prefix,
			&key.Fingerprint,
			&key.CreatedAt,
			&key.LastUsed,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, &key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating API keys: %w", err)
	}
	return keys, nil
}

func (r *AuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	query := `UPDATE api_keys SET last_used = $2 WHERE id = $1`
	if _, err := r.db.Exec(ctx, query, id, sql.NullTime{Time: time.Now(), Valid: true}); err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

func (r *AuthRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM api_keys WHERE id = $1", id); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

func (r *AuthRepo) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	user.Role = model.RoleAdmin
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	query := `
        INSERT INTO users (id, email, role, created_at)
        SELECT $1, $2, $3, $4
        WHERE NOT EXISTS (SELECT 1 FROM users WHERE role = $5)`
	tag, err := r.db.Exec(ctx, query, user.ID, user.Email, user.Role, user.CreatedAt, model.RoleAdmin)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return core.NewError(fmt.Errorf("system already bootstrapped"), "ALREADY_BOOTSTRAPPED", nil)
		}
		return fmt.Errorf("creating initial admin user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return core.NewError(fmt.Errorf("system already bootstrapped"), "ALREADY_BOOTSTRAPPED", nil)
	}
	return nil
}
