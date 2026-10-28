-- +goose Up
-- apply declarative trigger "automation_job_overlays_workspace_archived_insert_guard" on table "automation_job_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_job_overlays_workspace_archived_insert_guard
BEFORE INSERT ON automation_job_overlays
WHEN NEW.enabled_override = 1 AND EXISTS (
  SELECT 1 FROM automation_jobs a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = NEW.job_id AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_job_overlays_workspace_archived_update_guard" on table "automation_job_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_job_overlays_workspace_archived_update_guard
BEFORE UPDATE ON automation_job_overlays
WHEN NEW.enabled_override = 1 AND EXISTS (
  SELECT 1 FROM automation_jobs a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = NEW.job_id AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_jobs_workspace_archived_insert_guard" on table "automation_jobs"
-- +goose StatementBegin
CREATE TRIGGER automation_jobs_workspace_archived_insert_guard
BEFORE INSERT ON automation_jobs
WHEN COALESCE((SELECT enabled_override FROM automation_job_overlays WHERE job_id = NEW.id), NEW.enabled) = 1
  AND EXISTS (SELECT 1 FROM workspaces WHERE id IN (NEW.workspace_id, NEW.loop_workspace_id) AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_jobs_workspace_archived_update_guard" on table "automation_jobs"
-- +goose StatementBegin
CREATE TRIGGER automation_jobs_workspace_archived_update_guard
BEFORE UPDATE OF workspace_id, loop_workspace_id, enabled ON automation_jobs
WHEN COALESCE((SELECT enabled_override FROM automation_job_overlays WHERE job_id = NEW.id), NEW.enabled) = 1
  AND EXISTS (SELECT 1 FROM workspaces WHERE id IN (NEW.workspace_id, NEW.loop_workspace_id) AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_trigger_overlays_workspace_archived_insert_guard" on table "automation_trigger_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_trigger_overlays_workspace_archived_insert_guard
BEFORE INSERT ON automation_trigger_overlays
WHEN NEW.enabled_override = 1 AND EXISTS (
  SELECT 1 FROM automation_triggers a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = NEW.trigger_id AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_trigger_overlays_workspace_archived_update_guard" on table "automation_trigger_overlays"
-- +goose StatementBegin
CREATE TRIGGER automation_trigger_overlays_workspace_archived_update_guard
BEFORE UPDATE ON automation_trigger_overlays
WHEN NEW.enabled_override = 1 AND EXISTS (
  SELECT 1 FROM automation_triggers a JOIN workspaces w ON w.id IN (a.workspace_id, a.loop_workspace_id)
  WHERE a.id = NEW.trigger_id AND w.archived_at IS NOT NULL
)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_triggers_workspace_archived_insert_guard" on table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_workspace_archived_insert_guard
BEFORE INSERT ON automation_triggers
WHEN COALESCE((SELECT enabled_override FROM automation_trigger_overlays WHERE trigger_id = NEW.id), NEW.enabled) = 1
  AND EXISTS (SELECT 1 FROM workspaces WHERE id IN (NEW.workspace_id, NEW.loop_workspace_id) AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "automation_triggers_workspace_archived_update_guard" on table "automation_triggers"
-- +goose StatementBegin
CREATE TRIGGER automation_triggers_workspace_archived_update_guard
BEFORE UPDATE OF workspace_id, loop_workspace_id, enabled ON automation_triggers
WHEN COALESCE((SELECT enabled_override FROM automation_trigger_overlays WHERE trigger_id = NEW.id), NEW.enabled) = 1
  AND EXISTS (SELECT 1 FROM workspaces WHERE id IN (NEW.workspace_id, NEW.loop_workspace_id) AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "loop_runs_workspace_archived_insert_guard" on table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_workspace_archived_insert_guard
BEFORE INSERT ON loop_runs
WHEN NEW.status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
  AND EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "loop_runs_workspace_archived_update_guard" on table "loop_runs"
-- +goose StatementBegin
CREATE TRIGGER loop_runs_workspace_archived_update_guard
BEFORE UPDATE OF workspace_id, status ON loop_runs
WHEN NEW.status IN ('queued', 'running', 'watching', 'needs-approval', 'paused')
  AND EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "sessions_workspace_archived_insert_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_archived_insert_guard
BEFORE INSERT ON sessions
WHEN NEW.state <> 'stopped'
  AND EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
-- apply declarative trigger "sessions_workspace_archived_update_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER sessions_workspace_archived_update_guard
BEFORE UPDATE OF workspace_id, state ON sessions
WHEN NEW.state <> 'stopped'
  AND EXISTS (SELECT 1 FROM workspaces WHERE id = NEW.workspace_id AND archived_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'workspace archived');
END;
-- +goose StatementEnd
