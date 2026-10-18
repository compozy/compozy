-- +goose Up

-- Profile selection became one choice shared by every workspace (#792). Seed the Global slot from
-- the most recently remembered workspace choice when none exists, so upgrading never drops a user
-- back to default. Workspace rows are kept; the CLI still honors them until v0.5.0.
INSERT INTO profile_selections (lens, workspace_id, profile_id, updated_at)
SELECT 'global', '', profile_id, updated_at
FROM profile_selections
WHERE lens = 'workspace'
  AND NOT EXISTS (SELECT 1 FROM profile_selections WHERE lens = 'global')
ORDER BY updated_at DESC, workspace_id
LIMIT 1;
