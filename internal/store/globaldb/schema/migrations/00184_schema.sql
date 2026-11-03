-- +goose Up
-- add column "failure_seen_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `failure_seen_at` text NULL;
-- drop index "idx_sessions_global_catalog_attention" from table: "sessions"
DROP INDEX `idx_sessions_global_catalog_attention`;
-- create index "idx_sessions_global_catalog_attention" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_attention
 ON sessions(archived_at, (CASE
 WHEN (stop_verification_failed = 1 AND state <> 'stopped')
 OR pending_permission_count > 0 OR pending_clarify_count > 0
 OR (trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') AND state <> 'stopped')
 OR (state = 'stopped' AND trim(COALESCE(failure_kind, '')) <> '' AND trim(COALESCE(failure_kind, '')) <> 'cancellation'
 AND failure_seen_at IS NULL)
 OR (state = 'stopped' AND later_returned_at IS NOT NULL)
 OR (state = 'active' AND ended_early_turn_id <> '' AND trim(COALESCE(stall_state, '')) <> 'stalled'
 AND trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) = '') THEN 0
 WHEN state = 'active' AND trim(COALESCE(stall_state, '')) <> 'stalled'
 AND trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) = ''
 AND last_settled_revision > last_seen_revision THEN 1
 ELSE 2 END), COALESCE(attention_changed_at, updated_at) DESC, updated_at DESC, created_at DESC, id DESC);
-- drop index "idx_sessions_global_catalog_navigator" from table: "sessions"
DROP INDEX `idx_sessions_global_catalog_navigator`;
-- create index "idx_sessions_global_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_navigator
 ON sessions(archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation')
 THEN (CASE WHEN failure_seen_at IS NULL OR later_returned_at IS NOT NULL THEN 0 ELSE 3 END)
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' AND later_returned_at IS NOT NULL THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND ended_early_turn_id <> '' THEN 0
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- drop index "idx_sessions_profile_global_catalog_navigator" from table: "sessions"
DROP INDEX `idx_sessions_profile_global_catalog_navigator`;
-- create index "idx_sessions_profile_global_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_profile_global_catalog_navigator
 ON sessions(profile_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation')
 THEN (CASE WHEN failure_seen_at IS NULL OR later_returned_at IS NOT NULL THEN 0 ELSE 3 END)
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' AND later_returned_at IS NOT NULL THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND ended_early_turn_id <> '' THEN 0
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- drop index "idx_sessions_workspace_catalog_navigator" from table: "sessions"
DROP INDEX `idx_sessions_workspace_catalog_navigator`;
-- create index "idx_sessions_workspace_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_workspace_catalog_navigator
 ON sessions(workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation')
 THEN (CASE WHEN failure_seen_at IS NULL OR later_returned_at IS NOT NULL THEN 0 ELSE 3 END)
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' AND later_returned_at IS NOT NULL THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND ended_early_turn_id <> '' THEN 0
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- drop index "idx_sessions_profile_workspace_catalog_navigator" from table: "sessions"
DROP INDEX `idx_sessions_profile_workspace_catalog_navigator`;
-- create index "idx_sessions_profile_workspace_catalog_navigator" to table: "sessions"
CREATE INDEX idx_sessions_profile_workspace_catalog_navigator
 ON sessions(profile_id, workspace_id, archived_at, (CASE
 WHEN stop_verification_failed = 1 AND state <> 'stopped' THEN 0
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation')
 THEN (CASE WHEN failure_seen_at IS NULL OR later_returned_at IS NOT NULL THEN 0 ELSE 3 END)
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' AND later_returned_at IS NOT NULL THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND ended_early_turn_id <> '' THEN 0
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- apply declarative trigger "trg_sessions_failure_seen_reset" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_failure_seen_reset
			AFTER UPDATE OF state, failure_kind, failure_summary ON sessions
			WHEN OLD.failure_seen_at IS NOT NULL AND (NEW.state IS NOT OLD.state
				OR NEW.failure_kind IS NOT OLD.failure_kind OR NEW.failure_summary IS NOT OLD.failure_summary)
			BEGIN
				UPDATE sessions SET failure_seen_at = NULL WHERE id = NEW.id;
			END;
-- +goose StatementEnd
