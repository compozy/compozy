-- +goose Up
ALTER TABLE automation_job_catalog_entries ADD COLUMN target TEXT NOT NULL DEFAULT '';
ALTER TABLE automation_trigger_catalog_entries ADD COLUMN target TEXT NOT NULL DEFAULT '';
UPDATE automation_job_catalog_entries SET target = (
  SELECT CASE WHEN j.task IS NOT NULL AND j.task <> '' AND j.task <> 'null' THEN 'task' ELSE j.target_kind END
  FROM automation_jobs AS j WHERE j.id = job_id
);
UPDATE automation_trigger_catalog_entries SET target = (
  SELECT t.target_kind FROM automation_triggers AS t WHERE t.id = trigger_id
);
CREATE INDEX idx_automation_runs_job_latest ON automation_runs(job_id, started_at DESC, id DESC) WHERE job_id IS NOT NULL;
CREATE INDEX idx_automation_runs_trigger_latest ON automation_runs(trigger_id, started_at DESC, id DESC) WHERE trigger_id IS NOT NULL;

-- +goose Down
DROP INDEX idx_automation_runs_trigger_latest;
DROP INDEX idx_automation_runs_job_latest;
ALTER TABLE automation_trigger_catalog_entries DROP COLUMN target;
ALTER TABLE automation_job_catalog_entries DROP COLUMN target;
