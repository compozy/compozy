-- +goose Up
-- +goose StatementBegin

-- Add org_id to workflow_states table
ALTER TABLE workflow_states
ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

-- Set default org_id for existing records to system organization
UPDATE workflow_states
SET org_id = '00000000-0000-0000-0000-000000000000'::UUID
WHERE org_id IS NULL;

-- Make org_id NOT NULL after setting defaults
ALTER TABLE workflow_states
ALTER COLUMN org_id SET NOT NULL;

-- Add org_id to task_states table
ALTER TABLE task_states
ADD COLUMN IF NOT EXISTS org_id UUID REFERENCES organizations(id) ON DELETE CASCADE;

-- Set default org_id for existing records to system organization
UPDATE task_states
SET org_id = '00000000-0000-0000-0000-000000000000'::UUID
WHERE org_id IS NULL;

-- Make org_id NOT NULL after setting defaults
ALTER TABLE task_states
ALTER COLUMN org_id SET NOT NULL;

-- Add multi-tenant indexes for performance
CREATE INDEX IF NOT EXISTS idx_workflow_states_org_id ON workflow_states (org_id);
CREATE INDEX IF NOT EXISTS idx_workflow_states_org_created ON workflow_states (org_id, created_at);
CREATE INDEX IF NOT EXISTS idx_workflow_states_org_status ON workflow_states (org_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_states_org_workflow_id ON workflow_states (org_id, workflow_id);

CREATE INDEX IF NOT EXISTS idx_task_states_org_id ON task_states (org_id);
CREATE INDEX IF NOT EXISTS idx_task_states_org_created ON task_states (org_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_states_org_status ON task_states (org_id, status);
CREATE INDEX IF NOT EXISTS idx_task_states_org_workflow_id ON task_states (org_id, workflow_id);
CREATE INDEX IF NOT EXISTS idx_task_states_org_task_id ON task_states (org_id, task_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Only proceed if tables exist
DO $$
BEGIN
    -- Remove task_states indexes and column if table exists
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'task_states') THEN
        DROP INDEX IF EXISTS idx_task_states_org_task_id;
        DROP INDEX IF EXISTS idx_task_states_org_workflow_id;
        DROP INDEX IF EXISTS idx_task_states_org_status;
        DROP INDEX IF EXISTS idx_task_states_org_created;
        DROP INDEX IF EXISTS idx_task_states_org_id;
        
        ALTER TABLE task_states DROP COLUMN IF EXISTS org_id;
    END IF;
    
    -- Remove workflow_states indexes and column if table exists
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'workflow_states') THEN
        DROP INDEX IF EXISTS idx_workflow_states_org_workflow_id;
        DROP INDEX IF EXISTS idx_workflow_states_org_status;
        DROP INDEX IF EXISTS idx_workflow_states_org_created;
        DROP INDEX IF EXISTS idx_workflow_states_org_id;
        
        ALTER TABLE workflow_states DROP COLUMN IF EXISTS org_id;
    END IF;
END $$;

-- +goose StatementEnd
