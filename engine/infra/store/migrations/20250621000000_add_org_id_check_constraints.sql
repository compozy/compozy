-- +goose Up
-- +goose StatementBegin

-- Add CHECK constraints to enforce non-empty organization IDs
-- This provides defense-in-depth at the database level to prevent any application
-- bugs from allowing cross-tenant data access or data without proper tenant isolation

-- Workflow states table
-- NOTE: 'system' KSUID is allowed as it represents the system organization
ALTER TABLE workflow_states 
ADD CONSTRAINT workflow_states_org_id_not_empty 
CHECK (org_id IS NOT NULL);

-- Task states table
-- NOTE: 'system' KSUID is allowed as it represents the system organization
ALTER TABLE task_states 
ADD CONSTRAINT task_states_org_id_not_empty 
CHECK (org_id IS NOT NULL);

-- Organizations table
-- NOTE: Skipping empty check for organizations table as it uses 
-- 'system' as the system organization ID
ALTER TABLE organizations 
ADD CONSTRAINT organizations_id_not_null 
CHECK (id IS NOT NULL);

-- Users table
-- NOTE: 'system' KSUID is allowed for system admin users
ALTER TABLE users 
ADD CONSTRAINT users_org_id_not_empty 
CHECK (org_id IS NOT NULL);

-- API keys table
-- NOTE: 'system' KSUID is allowed for system-level API keys
ALTER TABLE api_keys 
ADD CONSTRAINT api_keys_org_id_not_empty 
CHECK (org_id IS NOT NULL);

-- Add composite indexes to optimize tenant-isolated queries
-- These indexes ensure efficient filtering by org_id in all common query patterns

-- Workflow states - optimize queries by workflow_id within an org
CREATE INDEX IF NOT EXISTS idx_workflow_states_org_workflow 
ON workflow_states(org_id, workflow_id);

-- Task states - optimize queries by task_id within an org
CREATE INDEX IF NOT EXISTS idx_task_states_org_task 
ON task_states(org_id, task_id);

-- Task states - optimize queries by workflow_exec_id within an org
CREATE INDEX IF NOT EXISTS idx_task_states_org_workflow_exec 
ON task_states(org_id, workflow_exec_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop indexes first
DROP INDEX IF EXISTS idx_task_states_org_workflow_exec;
DROP INDEX IF EXISTS idx_task_states_org_task;
DROP INDEX IF EXISTS idx_workflow_states_org_workflow;

-- Drop CHECK constraints
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_org_id_not_empty;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_org_id_not_empty;
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_id_not_null;
ALTER TABLE task_states DROP CONSTRAINT IF EXISTS task_states_org_id_not_empty;
ALTER TABLE workflow_states DROP CONSTRAINT IF EXISTS workflow_states_org_id_not_empty;

-- +goose StatementEnd
