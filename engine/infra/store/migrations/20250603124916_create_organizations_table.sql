-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    temporal_namespace VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'provisioning',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT organizations_name_check CHECK (LENGTH(name) >= 2),
    CONSTRAINT organizations_status_check CHECK (status IN ('provisioning', 'active', 'suspended', 'provisioning_failed')),
    CONSTRAINT organizations_temporal_namespace_check CHECK (LENGTH(temporal_namespace) >= 3)
);

-- Unique constraints
CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_name ON organizations (name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_temporal_namespace ON organizations (temporal_namespace);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations (status);
CREATE INDEX IF NOT EXISTS idx_organizations_created_at ON organizations (created_at);
CREATE INDEX IF NOT EXISTS idx_organizations_status_created ON organizations (status, created_at);

-- Create system organization for existing data
INSERT INTO organizations (
    id,
    name,
    temporal_namespace,
    status,
    created_at,
    updated_at
) VALUES (
    'system',
    'system',
    'compozy-system',
    'active',
    NOW(),
    NOW()
) ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_organizations_status_created;
DROP INDEX IF EXISTS idx_organizations_created_at;
DROP INDEX IF EXISTS idx_organizations_status;
DROP INDEX IF EXISTS idx_organizations_temporal_namespace;
DROP INDEX IF EXISTS idx_organizations_name;
DROP TABLE IF EXISTS organizations;
-- +goose StatementEnd
