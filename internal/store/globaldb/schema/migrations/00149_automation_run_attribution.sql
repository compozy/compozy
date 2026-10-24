-- +goose Up
UPDATE automation_runs
SET workspace_id = COALESCE((SELECT workspace_id FROM automation_jobs WHERE id = automation_runs.job_id), ''),
    attribution = 'derived'
WHERE attribution = 'unknown' AND workspace_id IS NULL
  AND EXISTS (SELECT 1 FROM automation_jobs WHERE id = automation_runs.job_id);
UPDATE automation_runs
SET workspace_id = COALESCE((SELECT workspace_id FROM automation_triggers WHERE id = automation_runs.trigger_id), ''),
    attribution = 'derived'
WHERE attribution = 'unknown' AND workspace_id IS NULL
  AND EXISTS (SELECT 1 FROM automation_triggers WHERE id = automation_runs.trigger_id);

-- +goose Down
SELECT 1;
