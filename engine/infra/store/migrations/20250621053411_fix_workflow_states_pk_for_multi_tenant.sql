-- +goose Up
-- +goose StatementBegin

-- Fix workflow_states primary key to be (workflow_exec_id, org_id) 
-- This prevents cross-tenant workflow hijacking in ON CONFLICT clauses

-- First, drop any foreign key constraints that depend on the primary key
ALTER TABLE task_states DROP CONSTRAINT IF EXISTS fk_workflow;

-- Drop the existing primary key constraint
ALTER TABLE workflow_states DROP CONSTRAINT workflow_states_pkey;

-- Add the new composite primary key
ALTER TABLE workflow_states ADD CONSTRAINT workflow_states_pkey 
PRIMARY KEY (workflow_exec_id, org_id);

-- Recreate the foreign key constraint to reference the composite primary key
ALTER TABLE task_states ADD CONSTRAINT fk_workflow 
FOREIGN KEY (workflow_exec_id, org_id) REFERENCES workflow_states(workflow_exec_id, org_id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop the composite foreign key constraint first
ALTER TABLE task_states DROP CONSTRAINT IF EXISTS fk_workflow;

-- Restore the original single-column primary key
ALTER TABLE workflow_states DROP CONSTRAINT workflow_states_pkey;
ALTER TABLE workflow_states ADD CONSTRAINT workflow_states_pkey 
PRIMARY KEY (workflow_exec_id);

-- Recreate the original foreign key constraint
ALTER TABLE task_states ADD CONSTRAINT fk_workflow 
FOREIGN KEY (workflow_exec_id) REFERENCES workflow_states(workflow_exec_id) ON DELETE CASCADE;

-- +goose StatementEnd
