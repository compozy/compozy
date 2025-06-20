# Multi-Tenancy Implementation Guide

## Overview

This document describes the multi-tenancy patterns implemented in the Compozy authentication system.

## Data Isolation Strategy

### 1. Query-Level Isolation

All repository methods enforce organization-level data isolation:

```go
// User repository example
func (r *postgresRepository) GetByID(ctx context.Context, orgID, userID core.ID) (*User, error) {
    query := `
        SELECT id, org_id, email, role, status, created_at, updated_at
        FROM users
        WHERE org_id = $1 AND id = $2
    `
    // org_id is always required and filtered
}
```

### 2. Foreign Key Constraints

Database schema enforces referential integrity:

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- other fields
);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- other fields
);
```

### 3. Validation Triggers

API keys must belong to users in the same organization:

```sql
CREATE OR REPLACE FUNCTION validate_api_key_org()
RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM users
        WHERE id = NEW.user_id
        AND org_id = NEW.org_id
    ) THEN
        RAISE EXCEPTION 'API key org_id must match user org_id';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

## Connection Pooling Configuration

### Current Settings

```go
config.MaxConns = 20        // Maximum connections in pool
config.MinConns = 2         // Minimum idle connections
config.HealthCheckPeriod = 30 * time.Second
config.ConnConfig.ConnectTimeout = 5 * time.Second
```

### Multi-Tenant Considerations

1. **Shared Pool**: All organizations share the same connection pool for efficiency
2. **Query Isolation**: Data isolation is enforced at the query level, not connection level
3. **Performance**: Connection pool size should be tuned based on:
    - Number of active organizations
    - Concurrent users per organization
    - Query patterns and duration

### Recommended Monitoring

For production multi-tenant systems, consider monitoring:

- Queries per organization
- Connection pool utilization by organization
- Slow queries by organization
- API rate limiting per organization

## Transaction Patterns

### Cross-Repository Transactions

All repositories support transactional operations across organization boundaries:

```go
err := db.WithTx(ctx, func(tx pgx.Tx) error {
    // All repositories use the same transaction
    txOrgRepo := orgRepo.WithTx(tx)
    txUserRepo := userRepo.WithTx(tx)
    txAPIKeyRepo := apiKeyRepo.WithTx(tx)

    // Operations are atomic within the organization
    return nil
})
```

## Security Best Practices

1. **Always Include org_id**: Never query without organization context
2. **Validate Ownership**: Ensure resources belong to the requesting organization
3. **Audit Trails**: Log all cross-organization access attempts
4. **Rate Limiting**: Implement per-organization rate limits
5. **Resource Quotas**: Enforce limits on users/API keys per organization

## Testing Multi-Tenancy

When testing, always verify:

1. Data isolation between organizations
2. Cascade deletes work correctly
3. Foreign key constraints are enforced
4. Triggers validate cross-table relationships
5. Queries cannot access data from other organizations

## Future Enhancements

Consider these additions for enhanced multi-tenancy:

1. **Row-Level Security (RLS)**: PostgreSQL RLS policies for defense in depth
2. **Schema Separation**: Separate schemas per organization for larger deployments
3. **Read Replicas**: Organization-specific read replicas for scale
4. **Caching Strategy**: Organization-aware caching with proper invalidation
5. **Metrics & Monitoring**: Per-organization performance tracking
