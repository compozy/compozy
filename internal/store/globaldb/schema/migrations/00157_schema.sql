-- +goose Up
-- apply declarative trigger "automation_job_overlays_workspace_archived_delete_guard" on table "automation_job_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_job_overlays_workspace_archived_delete_guard
BEFORE DELETE ON automation_job_overlays
WHEN EXISTS (
  SELECT 1 FROM automation_jobs a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = OLD.job_id AND a.enabled = 1 AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_trigger_overlays_workspace_archived_delete_guard" on table "automation_trigger_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_trigger_overlays_workspace_archived_delete_guard
BEFORE DELETE ON automation_trigger_overlays
WHEN EXISTS (
  SELECT 1 FROM automation_triggers a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = OLD.trigger_id AND a.enabled = 1 AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
