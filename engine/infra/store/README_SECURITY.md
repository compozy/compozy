# Security Guidelines for Organization-Scoped Data Access

## Critical Security Fix: Insert Operations

A critical vulnerability was discovered in the `FilterInsertByOrganization` method where the organization ID (org_id) was not being properly enforced for INSERT operations. This could allow data to be created without proper tenant isolation.

### The Vulnerability

The original implementation returned columns and values without mutating the InsertBuilder, meaning if a caller didn't properly use the returned values, rows could be created without org_id, completely bypassing tenant isolation.

### The Fix

We've implemented two secure approaches for handling INSERT operations with automatic org_id enforcement:

#### Approach 1: BuildOrgAwareInsert (Recommended)

Use `BuildOrgAwareInsert` to create a new InsertBuilder with org_id pre-configured:

```go
// Create an InsertBuilder with org_id column already included
ib, orgID, err := queryFilter.BuildOrgAwareInsert(ctx, "table_name")
if err != nil {
    return fmt.Errorf("building insert: %w", err)
}

// Add your columns and values (remember to include orgID in values)
ib = ib.Columns("name", "email").Values(orgID, "John", "john@example.com")

// Execute the query
sql, args, err := ib.ToSql()
if err != nil {
    return fmt.Errorf("building SQL: %w", err)
}
_, err = db.Exec(ctx, sql, args...)
```

#### Approach 2: EnforceOrgIDInValues

Use `EnforceOrgIDInValues` when you have columns and values arrays:

```go
columns := []string{"name", "email", "status"}
values := []any{"John", "john@example.com", "active"}

// Enforce org_id is included
columns, values, err := queryFilter.EnforceOrgIDInValues(ctx, columns, values)
if err != nil {
    return fmt.Errorf("enforcing org_id: %w", err)
}

// Now columns includes "org_id" and values includes the org ID from context
query := fmt.Sprintf("INSERT INTO users (%s) VALUES (%s)",
    strings.Join(columns, ", "),
    strings.Repeat("?, ", len(values)-1) + "?")
```

### Best Practices

1. **Always use QueryFilters methods** for organization-scoped operations
2. **Never build INSERT queries manually** without enforcing org_id
3. **Use PreventCrossOrgAccess** to validate org_id before operations
4. **Include org_id in all table schemas** that require tenant isolation

### Example: Secure Repository Implementation

```go
func (r *UserRepo) Create(ctx context.Context, user *User) error {
    // Validate organization access
    if err := r.queryFilter.PreventCrossOrgAccess(ctx, user.OrgID); err != nil {
        return fmt.Errorf("organization access validation failed: %w", err)
    }

    // Build insert with automatic org_id
    ib, orgID, err := r.queryFilter.BuildOrgAwareInsert(ctx, "users")
    if err != nil {
        return err
    }

    // Add columns and values (org_id is already in columns)
    ib = ib.Columns("name", "email", "role").
        Values(orgID, user.Name, user.Email, user.Role)

    sql, args, err := ib.ToSql()
    if err != nil {
        return fmt.Errorf("building query: %w", err)
    }

    _, err = r.db.Exec(ctx, sql, args...)
    return err
}
```

### Testing

Always test organization isolation:

```go
func TestOrgIsolation(t *testing.T) {
    ctx1 := store.WithOrganizationID(context.Background(), "org1")
    ctx2 := store.WithOrganizationID(context.Background(), "org2")

    // Create data in org1 context
    err := repo.Create(ctx1, &User{Name: "John", OrgID: "org1"})
    require.NoError(t, err)

    // Try to access from org2 context - should not find the user
    users, err := repo.List(ctx2, nil)
    require.NoError(t, err)
    assert.Empty(t, users, "should not see data from other organizations")
}
```

## Summary

The vulnerability has been fixed by:

1. Removing the unsafe `FilterInsertByOrganization` method
2. Adding `BuildOrgAwareInsert` for creating secure InsertBuilders
3. Adding `EnforceOrgIDInValues` for ensuring org_id in value arrays
4. Providing clear patterns for secure insert operations

All INSERT operations must now explicitly handle org_id using one of the secure methods provided.
