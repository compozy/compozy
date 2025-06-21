-- +goose Up
-- +goose StatementBegin

-- Fix task_states primary key to be (task_exec_id, org_id) 
-- This prevents cross-tenant task hijacking in ON CONFLICT clauses

-- First, drop any foreign key constraints that depend on the primary key
ALTER TABLE task_states DROP CONSTRAINT IF EXISTS fk_parent_task;

-- Drop the existing primary key constraint
ALTER TABLE task_states DROP CONSTRAINT task_states_pkey;

-- Add the new composite primary key
ALTER TABLE task_states ADD CONSTRAINT task_states_pkey 
PRIMARY KEY (task_exec_id, org_id);

-- Recreate the parent task foreign key constraint to reference the composite primary key
ALTER TABLE task_states ADD CONSTRAINT fk_parent_task 
FOREIGN KEY (parent_state_id, org_id) REFERENCES task_states(task_exec_id, org_id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop the composite foreign key
ALTER TABLE task_states DROP CONSTRAINT IF EXISTS fk_parent_task;

-- Restore the original single-column primary key
ALTER TABLE task_states DROP CONSTRAINT task_states_pkey;
ALTER TABLE task_states ADD CONSTRAINT task_states_pkey 
PRIMARY KEY (task_exec_id);

-- Recreate the original foreign key constraint
ALTER TABLE task_states ADD CONSTRAINT fk_parent_task 
FOREIGN KEY (parent_state_id) REFERENCES task_states(task_exec_id) ON DELETE CASCADE;

-- +goose StatementEnd
