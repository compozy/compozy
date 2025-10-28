package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	sqliteDriver "modernc.org/sqlite"
	sqliteLib "modernc.org/sqlite/lib"
)

// AuthRepo implements the auth repository backed by SQLite.
type AuthRepo struct{ db *sql.DB }

// NewAuthRepo creates a SQLite-backed auth repository.
func NewAuthRepo(db *sql.DB) uc.Repository {
	return &AuthRepo{db: db}
}

// CreateUser inserts a new user row.
func (r *AuthRepo) CreateUser(ctx context.Context, user *model.User) error {
	if user == nil {
		return fmt.Errorf("sqlite auth: nil user provided")
	}
	createdAt := user.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	const query = `INSERT INTO users (id, email, role, created_at) VALUES (?, ?, ?, ?)`
	if _, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.Role,
		createdAt.Format(time.RFC3339Nano),
	); err != nil {
		if mapped := classifyUserError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("sqlite auth: create user: %w", err)
	}
	user.CreatedAt = createdAt
	return nil
}

// GetUserByID returns a user by identifier.
func (r *AuthRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	const query = `SELECT id, email, role, created_at FROM users WHERE id = ?`
	var (
		user      model.User
		createdAt string
	)
	err := r.db.QueryRowContext(ctx, query, id.String()).
		Scan(&user.ID, &user.Email, &user.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uc.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: get user by id: %w", err)
	}
	ts, perr := parseSQLiteTime(createdAt)
	if perr != nil {
		return nil, fmt.Errorf("sqlite auth: parse user created_at: %w", perr)
	}
	user.CreatedAt = ts
	return &user, nil
}

// GetUserByEmail loads a user by email (case-insensitive).
func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	const query = `SELECT id, email, role, created_at FROM users WHERE lower(email) = lower(?)`
	var (
		user      model.User
		createdAt string
	)
	err := r.db.QueryRowContext(ctx, query, email).
		Scan(&user.ID, &user.Email, &user.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uc.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: get user by email: %w", err)
	}
	ts, perr := parseSQLiteTime(createdAt)
	if perr != nil {
		return nil, fmt.Errorf("sqlite auth: parse user created_at: %w", perr)
	}
	user.CreatedAt = ts
	return &user, nil
}

// ListUsers returns all users ordered by creation date descending.
func (r *AuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	const query = `SELECT id, email, role, created_at FROM users ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var (
			user      model.User
			createdAt string
		)
		if err := rows.Scan(&user.ID, &user.Email, &user.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite auth: scan user row: %w", err)
		}
		ts, perr := parseSQLiteTime(createdAt)
		if perr != nil {
			return nil, fmt.Errorf("sqlite auth: parse user created_at: %w", perr)
		}
		user.CreatedAt = ts
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite auth: iterate users: %w", err)
	}
	return users, nil
}

// UpdateUser updates mutable user fields.
func (r *AuthRepo) UpdateUser(ctx context.Context, user *model.User) error {
	if user == nil {
		return fmt.Errorf("sqlite auth: nil user provided")
	}
	const query = `UPDATE users SET email = ?, role = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, user.Email, user.Role, user.ID.String())
	if err != nil {
		if mapped := classifyUserError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("sqlite auth: update user: %w", err)
	}
	affected, aerr := result.RowsAffected()
	if aerr != nil {
		return fmt.Errorf("sqlite auth: update user rows affected: %w", aerr)
	}
	if affected == 0 {
		return uc.ErrUserNotFound
	}
	return nil
}

// DeleteUser removes a user and cascades to API keys.
func (r *AuthRepo) DeleteUser(ctx context.Context, id core.ID) error {
	const query = `DELETE FROM users WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("sqlite auth: delete user: %w", err)
	}
	rows, aerr := result.RowsAffected()
	if aerr != nil {
		return fmt.Errorf("sqlite auth: delete user rows affected: %w", aerr)
	}
	if rows == 0 {
		return uc.ErrUserNotFound
	}
	return nil
}

// CreateAPIKey inserts a new API key.
func (r *AuthRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	if key == nil {
		return fmt.Errorf("sqlite auth: nil api key provided")
	}
	createdAt := key.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	const query = `
		INSERT INTO api_keys (id, user_id, hash, fingerprint, prefix, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		key.ID,
		key.UserID,
		key.Hash,
		key.Fingerprint,
		key.Prefix,
		createdAt.Format(time.RFC3339Nano),
	)
	if err := classifyAPIKeyError(err); err != nil {
		return err
	}
	key.CreatedAt = createdAt
	return nil
}

// GetAPIKeyByID fetches an API key by identifier.
func (r *AuthRepo) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	const query = `
		SELECT id, user_id, hash, fingerprint, prefix, created_at, last_used
		FROM api_keys
		WHERE id = ?
	`
	var (
		key       model.APIKey
		createdAt string
		lastUsed  sql.NullString
	)
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Fingerprint,
		&key.Prefix,
		&createdAt,
		&lastUsed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uc.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: get api key by id: %w", err)
	}
	ts, perr := parseSQLiteTime(createdAt)
	if perr != nil {
		return nil, fmt.Errorf("sqlite auth: parse api key created_at: %w", perr)
	}
	key.CreatedAt = ts
	lts, lerr := parseNullableSQLiteTime(lastUsed)
	if lerr != nil {
		return nil, fmt.Errorf("sqlite auth: parse api key last_used: %w", lerr)
	}
	key.LastUsed = lts
	return &key, nil
}

// GetAPIKeyByFingerprint fetches an API key by fingerprint.
func (r *AuthRepo) GetAPIKeyByFingerprint(ctx context.Context, fingerprint []byte) (*model.APIKey, error) {
	const query = `
		SELECT id, user_id, hash, fingerprint, prefix, created_at, last_used
		FROM api_keys
		WHERE fingerprint = ?
	`
	var (
		key       model.APIKey
		createdAt string
		lastUsed  sql.NullString
	)
	err := r.db.QueryRowContext(ctx, query, fingerprint).Scan(
		&key.ID,
		&key.UserID,
		&key.Hash,
		&key.Fingerprint,
		&key.Prefix,
		&createdAt,
		&lastUsed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, uc.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: get api key by fingerprint: %w", err)
	}
	ts, perr := parseSQLiteTime(createdAt)
	if perr != nil {
		return nil, fmt.Errorf("sqlite auth: parse api key created_at: %w", perr)
	}
	key.CreatedAt = ts
	lts, lerr := parseNullableSQLiteTime(lastUsed)
	if lerr != nil {
		return nil, fmt.Errorf("sqlite auth: parse api key last_used: %w", lerr)
	}
	key.LastUsed = lts
	return &key, nil
}

// ListAPIKeysByUserID lists API keys for a user ordered by creation date.
func (r *AuthRepo) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	const query = `
		SELECT id, user_id, hash, fingerprint, prefix, created_at, last_used
		FROM api_keys
		WHERE user_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID.String())
	if err != nil {
		return nil, fmt.Errorf("sqlite auth: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*model.APIKey
	for rows.Next() {
		var (
			key       model.APIKey
			createdAt string
			lastUsed  sql.NullString
		)
		if err := rows.Scan(
			&key.ID,
			&key.UserID,
			&key.Hash,
			&key.Fingerprint,
			&key.Prefix,
			&createdAt,
			&lastUsed,
		); err != nil {
			return nil, fmt.Errorf("sqlite auth: scan api key: %w", err)
		}
		ts, perr := parseSQLiteTime(createdAt)
		if perr != nil {
			return nil, fmt.Errorf("sqlite auth: parse api key created_at: %w", perr)
		}
		key.CreatedAt = ts
		lts, lerr := parseNullableSQLiteTime(lastUsed)
		if lerr != nil {
			return nil, fmt.Errorf("sqlite auth: parse api key last_used: %w", lerr)
		}
		key.LastUsed = lts
		keys = append(keys, &key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite auth: iterate api keys: %w", err)
	}
	return keys, nil
}

// UpdateAPIKeyLastUsed sets the last_used timestamp.
func (r *AuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	const query = `UPDATE api_keys SET last_used = ? WHERE id = ?`
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := r.db.ExecContext(ctx, query, now, id.String())
	if err != nil {
		return fmt.Errorf("sqlite auth: update api key last_used: %w", err)
	}
	rows, aerr := result.RowsAffected()
	if aerr != nil {
		return fmt.Errorf("sqlite auth: update api key rows affected: %w", aerr)
	}
	if rows == 0 {
		return uc.ErrAPIKeyNotFound
	}
	return nil
}

// DeleteAPIKey removes an API key.
func (r *AuthRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
	const query = `DELETE FROM api_keys WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("sqlite auth: delete api key: %w", err)
	}
	rows, aerr := result.RowsAffected()
	if aerr != nil {
		return fmt.Errorf("sqlite auth: delete api key rows affected: %w", aerr)
	}
	if rows == 0 {
		return uc.ErrAPIKeyNotFound
	}
	return nil
}

// CreateInitialAdminIfNone bootstraps the first admin user.
func (r *AuthRepo) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	if user == nil {
		return fmt.Errorf("sqlite auth: nil user provided")
	}
	createdAt := user.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	const query = `
		INSERT INTO users (id, email, role, created_at)
		SELECT ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE role = ?)
	`
	result, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		model.RoleAdmin,
		createdAt.Format(time.RFC3339Nano),
		model.RoleAdmin,
	)
	if err != nil {
		if mapped := classifyUserError(err); mapped != nil {
			return mapped
		}
		return fmt.Errorf("sqlite auth: create initial admin: %w", err)
	}
	rows, aerr := result.RowsAffected()
	if aerr != nil {
		return fmt.Errorf("sqlite auth: create initial admin rows affected: %w", aerr)
	}
	if rows == 0 {
		return core.NewError(fmt.Errorf("system already bootstrapped"), "ALREADY_BOOTSTRAPPED", nil)
	}
	user.Role = model.RoleAdmin
	user.CreatedAt = createdAt
	return nil
}

func classifyUserError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueConstraint(err) {
		lowerMsg := strings.ToLower(err.Error())
		if strings.Contains(lowerMsg, "users") || strings.Contains(lowerMsg, "idx_users_email_ci") {
			return uc.ErrEmailExists
		}
	}
	return nil
}

func classifyAPIKeyError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case isForeignKeyConstraint(err):
		return fmt.Errorf("sqlite auth: api key foreign key: %w", err)
	case isUniqueConstraint(err):
		return fmt.Errorf("sqlite auth: api key unique constraint: %w", err)
	default:
		return fmt.Errorf("sqlite auth: create api key: %w", err)
	}
}

func isUniqueConstraint(err error) bool {
	code, ok := sqliteErrorCode(err)
	if !ok {
		return false
	}
	switch code {
	case sqliteLib.SQLITE_CONSTRAINT,
		sqliteLib.SQLITE_CONSTRAINT_PRIMARYKEY,
		sqliteLib.SQLITE_CONSTRAINT_UNIQUE:
		return true
	default:
		return strings.Contains(err.Error(), "UNIQUE constraint failed")
	}
}

func isForeignKeyConstraint(err error) bool {
	code, ok := sqliteErrorCode(err)
	if !ok {
		return false
	}
	return code == sqliteLib.SQLITE_CONSTRAINT_FOREIGNKEY ||
		strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

func sqliteErrorCode(err error) (int, bool) {
	var sqlErr *sqliteDriver.Error
	if !errors.As(err, &sqlErr) {
		return 0, false
	}
	return sqlErr.Code(), true
}

func parseSQLiteTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time value")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}

func parseNullableSQLiteTime(value sql.NullString) (sql.NullTime, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return sql.NullTime{}, nil
	}
	ts, err := parseSQLiteTime(value.String)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: ts, Valid: true}, nil
}

// Ensure interface compliance at compile time.
var _ uc.Repository = (*AuthRepo)(nil)
