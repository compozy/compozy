## markdown

## status: completed

<task_context>
<domain>engine/infra/sqlite</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 2.0: Authentication Repository (SQLite)

## Overview

Implement SQLite-backed authentication repository for user and API key management. Port the PostgreSQL `authrepo.go` implementation to SQLite, handling user CRUD operations, API key management, and foreign key constraints.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** @tasks/prd-postgres/_techspec.md section on Auth Repository
- **ALWAYS READ** @tasks/prd-postgres/_tests.md for test requirements
- **DEPENDENCY:** Requires Task 1.0 (Foundation) complete
- **MANDATORY:** Use `database/sql` standard library (no pgx)
- **MANDATORY:** Case-insensitive email queries using `lower(email)`
- **MANDATORY:** Foreign key constraints enforced for API keys
</critical>

<requirements>
- Implement `AuthRepo` struct using `*sql.DB`
- Port all methods from `engine/infra/postgres/authrepo.go`
- User CRUD: Create, GetByID, GetByEmail, List, Update, Delete
- API key operations: Create, GetByHash, GetByPrefix, Delete
- Use `?` placeholders (not `$1`)
- Handle TEXT timestamps (ISO8601 format)
- Enforce foreign key CASCADE on API keys
</requirements>

## Subtasks

- [x] 2.1 Create `engine/infra/sqlite/authrepo.go` structure
- [x] 2.2 Implement user CRUD operations
- [x] 2.3 Implement API key operations
- [x] 2.4 Add helper functions for common queries
- [x] 2.5 Write unit tests for user operations
- [x] 2.6 Write unit tests for API key operations
- [x] 2.7 Write integration tests for cascade deletes

## Implementation Details

### Reference Implementation

**Source:** `engine/infra/postgres/authrepo.go`

### 2.1 Repository Structure

```go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "github.com/compozy/compozy/engine/auth/uc"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/auth/model"
)

type AuthRepo struct {
    db *sql.DB
}

func NewAuthRepo(db *sql.DB) uc.Repository {
    return &AuthRepo{db: db}
}
```

### 2.2 User Operations

```go
func (r *AuthRepo) CreateUser(ctx context.Context, user *model.User) error {
    query := `
        INSERT INTO users (id, email, role, created_at)
        VALUES (?, ?, ?, ?)
    `
    
    _, err := r.db.ExecContext(ctx, query,
        user.ID.String(),
        user.Email,
        user.Role,
        user.CreatedAt.Format(time.RFC3339),
    )
    if err != nil {
        return fmt.Errorf("create user: %w", err)
    }
    
    return nil
}

func (r *AuthRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
    query := `
        SELECT id, email, role, created_at
        FROM users
        WHERE id = ?
    `
    
    var user model.User
    var createdAt string
    
    err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
        &user.ID,
        &user.Email,
        &user.Role,
        &createdAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found: %w", core.ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get user by id: %w", err)
    }
    
    // Parse timestamp
    user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    
    return &user, nil
}

func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
    query := `
        SELECT id, email, role, created_at
        FROM users
        WHERE lower(email) = lower(?)
    `
    
    var user model.User
    var createdAt string
    
    err := r.db.QueryRowContext(ctx, query, email).Scan(
        &user.ID,
        &user.Email,
        &user.Role,
        &createdAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found: %w", core.ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get user by email: %w", err)
    }
    
    user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
    
    return &user, nil
}

func (r *AuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
    query := `
        SELECT id, email, role, created_at
        FROM users
        ORDER BY created_at DESC
    `
    
    rows, err := r.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("list users: %w", err)
    }
    defer rows.Close()
    
    var users []*model.User
    for rows.Next() {
        var user model.User
        var createdAt string
        
        if err := rows.Scan(&user.ID, &user.Email, &user.Role, &createdAt); err != nil {
            return nil, fmt.Errorf("scan user: %w", err)
        }
        
        user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
        users = append(users, &user)
    }
    
    return users, rows.Err()
}

func (r *AuthRepo) DeleteUser(ctx context.Context, id core.ID) error {
    query := `DELETE FROM users WHERE id = ?`
    
    result, err := r.db.ExecContext(ctx, query, id.String())
    if err != nil {
        return fmt.Errorf("delete user: %w", err)
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("user not found: %w", core.ErrNotFound)
    }
    
    return nil
}
```

### 2.3 API Key Operations

```go
func (r *AuthRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
    query := `
        INSERT INTO api_keys (id, user_id, hash, prefix, created_at)
        VALUES (?, ?, ?, ?, ?)
    `
    
    _, err := r.db.ExecContext(ctx, query,
        key.ID.String(),
        key.UserID.String(),
        key.Hash,
        key.Prefix,
        key.CreatedAt.Format(time.RFC3339),
    )
    if err != nil {
        return fmt.Errorf("create api key: %w", err)
    }
    
    return nil
}

func (r *AuthRepo) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
    query := `
        SELECT id, user_id, hash, prefix, created_at, last_used
        FROM api_keys
        WHERE hash = ?
    `
    
    var key model.APIKey
    var createdAt, lastUsed sql.NullString
    
    err := r.db.QueryRowContext(ctx, query, hash).Scan(
        &key.ID,
        &key.UserID,
        &key.Hash,
        &key.Prefix,
        &createdAt,
        &lastUsed,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("api key not found: %w", core.ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("get api key by hash: %w", err)
    }
    
    key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
    if lastUsed.Valid {
        key.LastUsed, _ = time.Parse(time.RFC3339, lastUsed.String)
    }
    
    return &key, nil
}

func (r *AuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
    query := `
        UPDATE api_keys
        SET last_used = ?
        WHERE id = ?
    `
    
    _, err := r.db.ExecContext(ctx, query,
        time.Now().Format(time.RFC3339),
        id.String(),
    )
    if err != nil {
        return fmt.Errorf("update api key last used: %w", err)
    }
    
    return nil
}

func (r *AuthRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
    query := `DELETE FROM api_keys WHERE id = ?`
    
    result, err := r.db.ExecContext(ctx, query, id.String())
    if err != nil {
        return fmt.Errorf("delete api key: %w", err)
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("api key not found: %w", core.ErrNotFound)
    }
    
    return nil
}
```

### Relevant Files

**New Files:**
- `engine/infra/sqlite/authrepo.go`
- `engine/infra/sqlite/authrepo_test.go`

**Reference Files:**
- `engine/infra/postgres/authrepo.go` - Source implementation
- `engine/auth/uc/repository.go` - Interface definition
- `engine/auth/model/user.go` - User model
- `engine/auth/model/api_key.go` - API key model

### Dependent Files

- `engine/infra/sqlite/store.go` - Database connection (from Task 1.0)
- `engine/infra/sqlite/migrations/*.sql` - Schema (from Task 1.0)

## Deliverables

- [ ] `engine/infra/sqlite/authrepo.go` with complete implementation
- [ ] All user CRUD operations working
- [ ] All API key operations working
- [ ] Case-insensitive email queries implemented
- [ ] Foreign key constraints respected
- [ ] All unit tests passing
- [ ] All integration tests passing
- [ ] Code passes linting

## Tests

### Unit Tests (`engine/infra/sqlite/authrepo_test.go`)

- [ ] `TestAuthRepo/Should_create_user_successfully`
- [ ] `TestAuthRepo/Should_get_user_by_id`
- [ ] `TestAuthRepo/Should_get_user_by_email_case_insensitive`
- [ ] `TestAuthRepo/Should_return_error_for_duplicate_email`
- [ ] `TestAuthRepo/Should_list_all_users`
- [ ] `TestAuthRepo/Should_delete_user`
- [ ] `TestAuthRepo/Should_create_api_key`
- [ ] `TestAuthRepo/Should_get_api_key_by_hash`
- [ ] `TestAuthRepo/Should_update_api_key_last_used`
- [ ] `TestAuthRepo/Should_delete_api_key`
- [ ] `TestAuthRepo/Should_cascade_delete_api_keys_when_user_deleted`
- [ ] `TestAuthRepo/Should_enforce_foreign_key_constraint`

### Edge Cases

- [ ] `TestAuthRepo/Should_handle_missing_user_gracefully`
- [ ] `TestAuthRepo/Should_handle_missing_api_key_gracefully`
- [ ] `TestAuthRepo/Should_reject_invalid_foreign_key`
- [ ] `TestAuthRepo/Should_handle_null_last_used_timestamp`

## Success Criteria

- [ ] All user CRUD operations work correctly
- [ ] Email queries are case-insensitive
- [ ] API keys correctly reference users via foreign key
- [ ] Cascade delete removes API keys when user deleted
- [ ] Foreign key violations rejected appropriately
- [ ] Timestamps stored and retrieved correctly (ISO8601 format)
- [ ] All tests pass: `go test ./engine/infra/sqlite/authrepo_test.go`
- [ ] Code passes linting: `golangci-lint run ./engine/infra/sqlite/authrepo.go`
- [ ] Repository implements `uc.Repository` interface correctly
