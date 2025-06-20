package user

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

// scanUser is a helper function to scan a database row into a User struct
func scanUser(scannable interface{ Scan(dest ...any) error }) (*User, error) {
	var user User
	err := scannable.Scan(
		&user.ID,
		&user.OrgID,
		&user.Email,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Create creates a new user
func (r *postgresRepository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, org_id, email, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		user.ID,
		user.OrgID,
		user.Email,
		user.Role,
		user.Status,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by its ID within an organization
func (r *postgresRepository) GetByID(ctx context.Context, orgID, userID core.ID) (*User, error) {
	query := `
		SELECT id, org_id, email, role, status, created_at, updated_at
		FROM users
		WHERE org_id = $1 AND id = $2
	`
	user, err := scanUser(r.db.QueryRow(ctx, query, orgID, userID))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}

// GetByEmail retrieves a user by email within an organization
func (r *postgresRepository) GetByEmail(ctx context.Context, orgID core.ID, email string) (*User, error) {
	query := `
		SELECT id, org_id, email, role, status, created_at, updated_at
		FROM users
		WHERE org_id = $1 AND email = $2
	`
	user, err := scanUser(r.db.QueryRow(ctx, query, orgID, email))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// Update updates an existing user
func (r *postgresRepository) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET email = $3, role = $4, status = $5, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query,
		user.OrgID,
		user.ID,
		user.Email,
		user.Role,
		user.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Delete deletes a user by its ID within an organization
func (r *postgresRepository) Delete(ctx context.Context, orgID, userID core.ID) error {
	query := `DELETE FROM users WHERE org_id = $1 AND id = $2`
	result, err := r.db.Exec(ctx, query, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// List retrieves users within an organization with pagination
func (r *postgresRepository) List(ctx context.Context, orgID core.ID, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, org_id, email, role, status, created_at, updated_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}
	return users, nil
}

// ListByRole retrieves users by role within an organization
func (r *postgresRepository) ListByRole(
	ctx context.Context,
	orgID core.ID,
	role string,
	limit, offset int,
) ([]*User, error) {
	query := `
		SELECT id, org_id, email, role, status, created_at, updated_at
		FROM users
		WHERE org_id = $1 AND role = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, orgID, role, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by role: %w", err)
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}
	return users, nil
}

// UpdateRole updates the role of a user
func (r *postgresRepository) UpdateRole(ctx context.Context, orgID, userID core.ID, role string) error {
	query := `
		UPDATE users
		SET role = $3, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query, orgID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateStatus updates the status of a user
func (r *postgresRepository) UpdateStatus(ctx context.Context, orgID, userID core.ID, status Status) error {
	query := `
		UPDATE users
		SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE org_id = $1 AND id = $2
	`
	result, err := r.db.Exec(ctx, query, orgID, userID, status)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// FindByEmail searches for users by email pattern within an organization
func (r *postgresRepository) FindByEmail(ctx context.Context, orgID core.ID, emailPattern string) ([]*User, error) {
	query := `
		SELECT id, org_id, email, role, status, created_at, updated_at
		FROM users
		WHERE org_id = $1 AND email ILIKE $2
		ORDER BY email
	`
	rows, err := r.db.Query(ctx, query, orgID, "%"+emailPattern+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to find users by email: %w", err)
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}
	return users, nil
}

// CountByOrg returns the total count of users in an organization
func (r *postgresRepository) CountByOrg(ctx context.Context, orgID core.ID) (int64, error) {
	query := `SELECT COUNT(*) FROM users WHERE org_id = $1`
	var count int64
	err := r.db.QueryRow(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
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
