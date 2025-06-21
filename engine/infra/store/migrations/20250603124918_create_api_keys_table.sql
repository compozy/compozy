-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ,
    rate_limit_per_hour INTEGER NOT NULL DEFAULT 3600,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT api_keys_name_check CHECK (LENGTH(name) >= 1),
    CONSTRAINT api_keys_rate_limit_check CHECK (rate_limit_per_hour > 0 AND rate_limit_per_hour <= 10000),
    CONSTRAINT api_keys_status_check CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT api_keys_expires_check CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- Unique constraints
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (key_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys (key_prefix);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_user_name ON api_keys (user_id, name);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_id ON api_keys (org_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys (status);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys (expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_last_used ON api_keys (last_used_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_created ON api_keys (org_id, created_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_org_status ON api_keys (org_id, status);

-- Prefix index for key lookup optimization
CREATE INDEX IF NOT EXISTS idx_api_keys_hash_prefix ON api_keys (substring(key_hash, 1, 16));

-- Function to validate api_key org_id matches user org_id
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

-- Trigger to enforce api_key org constraint
CREATE TRIGGER api_keys_org_check
    BEFORE INSERT OR UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION validate_api_key_org();

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to enforce updated_at update
CREATE TRIGGER api_keys_set_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS api_keys_set_updated_at ON api_keys;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TRIGGER IF EXISTS api_keys_org_check ON api_keys;
DROP FUNCTION IF EXISTS validate_api_key_org();
DROP INDEX IF EXISTS idx_api_keys_hash_prefix;
DROP INDEX IF EXISTS idx_api_keys_org_status;
DROP INDEX IF EXISTS idx_api_keys_org_created;
DROP INDEX IF EXISTS idx_api_keys_last_used;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_status;
DROP INDEX IF EXISTS idx_api_keys_org_id;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_user_name;
DROP INDEX IF EXISTS idx_api_keys_prefix;
DROP INDEX IF EXISTS idx_api_keys_hash;
DROP TABLE IF EXISTS api_keys;
-- +goose StatementEnd
