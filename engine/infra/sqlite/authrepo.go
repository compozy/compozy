package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// AuthRepo implements uc.Repository on top of a SQLite *sql.DB.
type AuthRepo struct{ db *sql.DB }

// NewAuthRepo creates a new SQLite-backed auth repository.
func NewAuthRepo(db *sql.DB) uc.Repository { return &AuthRepo{db: db} }

// --- Users ---

func (r *AuthRepo) CreateUser(ctx context.Context, user *model.User) error {
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	const q = `INSERT INTO users (id, email, role, created_at) VALUES (?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, q, user.ID, user.Email, user.Role, user.CreatedAt); err != nil {
		return fmt.Errorf("sqlite: create user: %w", err)
	}
	return nil
}

func (r *AuthRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	const q = `SELECT id, email, role, created_at FROM users WHERE id = ?`
	var u model.User
	if err := r.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uc.ErrUserNotFound
		}
		return nil, fmt.Errorf("sqlite: get user by id: %w", err)
	}
	return &u, nil
}

func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `SELECT id, email, role, created_at FROM users WHERE lower(email) = lower(?)`
	var u model.User
	if err := r.db.QueryRowContext(ctx, q, email).Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uc.ErrUserNotFound
		}
		return nil, fmt.Errorf("sqlite: get user by email: %w", err)
	}
	return &u, nil
}

func (r *AuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	const q = `SELECT id, email, role, created_at FROM users ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list users: %w", err)
	}
	defer rows.Close()
	var out []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan user: %w", err)
		}
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iter users: %w", err)
	}
	return out, nil
}

func (r *AuthRepo) UpdateUser(ctx context.Context, user *model.User) error {
	const q = `UPDATE users SET email = ?, role = ?, updated_at = ? WHERE id = ?`
	tag, err := r.db.ExecContext(ctx, q, user.Email, user.Role, time.Now().UTC(), user.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update user: %w", err)
	}
	if n, raErr := tag.RowsAffected(); raErr == nil {
		if n == 0 {
			return uc.ErrUserNotFound
		}
	} else {
		return fmt.Errorf("sqlite: rows affected (update user): %w", raErr)
	}
	return nil
}

func (r *AuthRepo) DeleteUser(ctx context.Context, id core.ID) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rb := tx.Rollback(); rb != nil {
				logger.FromContext(ctx).Warn("sqlite: rollback failed", "error", rb)
			}
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM api_keys WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: delete user api_keys: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete user: %w", err)
	}
	if n, raErr := res.RowsAffected(); raErr == nil {
		if n == 0 {
			return uc.ErrUserNotFound
		}
	} else {
		return fmt.Errorf("sqlite: rows affected (delete user): %w", raErr)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit tx: %w", err)
	}
	return nil
}

// --- API Keys ---

func (r *AuthRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now().UTC()
	}
	const q = `INSERT INTO api_keys (id, user_id, hash, prefix, fingerprint, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, q,
		key.ID,
		key.UserID,
		key.Hash,
		key.Prefix,
		key.Fingerprint,
		key.CreatedAt,
	); err != nil {
		return fmt.Errorf("sqlite: create api key: %w", err)
	}
	return nil
}

func (r *AuthRepo) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	const q = `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE id = ?`
	var k model.APIKey
	if err := r.db.QueryRowContext(ctx, q, id).Scan(&k.ID, &k.UserID, &k.Hash, &k.Prefix, &k.Fingerprint, &k.CreatedAt, &k.LastUsed); err != nil { //nolint:lll // long scan kept for clarity
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uc.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("sqlite: get api key by id: %w", err)
	}
	return &k, nil
}

func (r *AuthRepo) GetAPIKeyByHash(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	const q = `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE fingerprint = ?`
	var k model.APIKey
	if err := r.db.QueryRowContext(ctx, q, fingerprint).Scan(&k.ID, &k.UserID, &k.Hash, &k.Prefix, &k.Fingerprint, &k.CreatedAt, &k.LastUsed); err != nil { //nolint:lll // long scan kept for clarity
		if errors.Is(err, sql.ErrNoRows) {
			return nil, uc.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("sqlite: get api key by hash: %w", err)
	}
	return &k, nil
}

func (r *AuthRepo) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	const q = `SELECT id, user_id, hash, prefix, fingerprint, created_at, last_used FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list api keys: %w", err)
	}
	defer rows.Close()
	var out []*model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Hash, &k.Prefix, &k.Fingerprint, &k.CreatedAt, &k.LastUsed); err != nil {
			return nil, fmt.Errorf("sqlite: scan api key: %w", err)
		}
		out = append(out, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iter api keys: %w", err)
	}
	return out, nil
}

func (r *AuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	// Use CASE to emulate GREATEST(last_used, now) in SQLite
	const q = `UPDATE api_keys SET last_used = CASE WHEN last_used IS NULL OR last_used < CURRENT_TIMESTAMP THEN CURRENT_TIMESTAMP ELSE last_used END WHERE id = ?`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("sqlite: update api key last_used: %w", err)
	}
	if n, raErr := res.RowsAffected(); raErr == nil {
		if n == 0 {
			return uc.ErrAPIKeyNotFound
		}
	} else {
		return fmt.Errorf("sqlite: rows affected (update api key): %w", raErr)
	}
	return nil
}

func (r *AuthRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete api key: %w", err)
	}
	if n, raErr := res.RowsAffected(); raErr == nil {
		if n == 0 {
			return uc.ErrAPIKeyNotFound
		}
	} else {
		return fmt.Errorf("sqlite: rows affected (delete api key): %w", raErr)
	}
	return nil
}

func (r *AuthRepo) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	user.Role = model.RoleAdmin
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	// Atomic insert-if-no-admin using INSERT ... SELECT ... WHERE NOT EXISTS
	const q = `
        INSERT INTO users (id, email, role, created_at)
        SELECT ?, ?, ?, ?
        WHERE NOT EXISTS (SELECT 1 FROM users WHERE role = 'admin')
    `
	res, err := r.db.ExecContext(ctx, q, user.ID, user.Email, user.Role, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("sqlite: create initial admin: %w", err)
	}
	if n, raErr := res.RowsAffected(); raErr == nil {
		if n == 0 {
			return core.NewError(fmt.Errorf("system already bootstrapped"), "ALREADY_BOOTSTRAPPED", nil)
		}
	} else {
		return fmt.Errorf("sqlite: rows affected (bootstrap admin): %w", raErr)
	}
	return nil
}
