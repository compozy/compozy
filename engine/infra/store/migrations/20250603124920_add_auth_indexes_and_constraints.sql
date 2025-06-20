-- +goose Up
-- +goose StatementBegin

-- Enable pg_trgm extension for efficient pattern matching
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Add unique constraints to enforce data integrity
CREATE UNIQUE INDEX IF NOT EXISTS organizations_name_uidx ON organizations(name);
CREATE UNIQUE INDEX IF NOT EXISTS users_org_email_uidx ON users(org_id, email);
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_org_prefix_uidx ON api_keys(org_id, key_prefix);

-- Add GIN indexes for efficient pattern matching on searched columns
CREATE INDEX IF NOT EXISTS idx_gin_organizations_name ON organizations USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_gin_users_email ON users USING gin (email gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_gin_api_keys_key_prefix ON api_keys USING gin (key_prefix gin_trgm_ops);

-- Add standard indexes for foreign key relationships and common queries
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_users_org_role ON users(org_id, role);
CREATE INDEX IF NOT EXISTS idx_users_org_status ON users(org_id, status);

CREATE INDEX IF NOT EXISTS idx_api_keys_org_id ON api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_user ON api_keys(org_id, user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_status ON api_keys(org_id, status);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- Add composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_api_keys_active_lookup ON api_keys(org_id, status, expires_at) 
WHERE status = 'active';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop indexes in reverse order
DROP INDEX IF EXISTS idx_api_keys_active_lookup;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_org_status;
DROP INDEX IF EXISTS idx_api_keys_org_user;
DROP INDEX IF EXISTS idx_api_keys_org_id;

DROP INDEX IF EXISTS idx_users_org_status;
DROP INDEX IF EXISTS idx_users_org_role;
DROP INDEX IF EXISTS idx_users_org_id;

-- Drop GIN indexes
DROP INDEX IF EXISTS idx_gin_api_keys_key_prefix;
DROP INDEX IF EXISTS idx_gin_users_email;
DROP INDEX IF EXISTS idx_gin_organizations_name;

-- Drop unique constraints
DROP INDEX IF EXISTS api_keys_org_prefix_uidx;
DROP INDEX IF EXISTS users_org_email_uidx;
DROP INDEX IF EXISTS organizations_name_uidx;

-- Note: We don't drop pg_trgm extension as it might be used elsewhere

-- +goose StatementEnd
