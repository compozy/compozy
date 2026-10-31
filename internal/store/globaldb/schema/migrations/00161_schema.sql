-- +goose Up
-- add column "parked_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `parked_at` text NULL;
-- add column "park_reason" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `park_reason` text NOT NULL DEFAULT '';
-- add column "park_until" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `park_until` text NULL;
-- add column "later_returned_at" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `later_returned_at` text NULL;
-- drop index "idx_sessions_global_catalog_attention" from table: "sessions"
DROP INDEX `idx_sessions_global_catalog_attention`;
-- create index "idx_sessions_global_catalog_attention" to table: "sessions"
CREATE INDEX idx_sessions_global_catalog_attention
 ON sessions(archived_at, (CASE
 WHEN (stop_verification_failed = 1 AND state <> 'stopped')
 OR pending_permission_count > 0 OR pending_clarify_count > 0
 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure')
 OR (state = 'stopped' AND trim(COALESCE(failure_kind, '')) <> '' AND trim(COALESCE(failure_kind, '')) <> 'cancellation')
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
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
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
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
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
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
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
 WHEN state = 'stopped' AND trim(COALESCE(failure_kind, '')) NOT IN ('', 'cancellation') THEN 0
 WHEN pending_permission_count > 0 OR trim(COALESCE(failure_kind, '')) IN ('permission_failure', 'provider_auth_failure') THEN 0
 WHEN pending_clarify_count > 0 THEN 0
 WHEN state = 'stopped' AND later_returned_at IS NOT NULL THEN 0
 WHEN state = 'stopped' THEN 3
 WHEN trim(COALESCE(stall_state, '')) = 'stalled' THEN 3
 WHEN state IN ('starting', 'stopping') OR trim(COALESCE(json_extract(CASE WHEN json_valid(activity_json) THEN activity_json ELSE '{}' END, '$.turn_id'), '')) <> '' THEN 2
 WHEN state = 'active' AND ended_early_turn_id <> '' THEN 0
 WHEN state = 'active' AND last_settled_revision > last_seen_revision THEN 1
 ELSE 3 END), COALESCE(attention_changed_at, updated_at) DESC, id ASC);
-- create index "idx_sessions_later_due" to table: "sessions"
CREATE INDEX `idx_sessions_later_due` ON `sessions` (`park_until`) WHERE parked_at IS NOT NULL AND park_until IS NOT NULL;
-- apply declarative trigger "trg_sessions_later_insert_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_later_insert_guard
			BEFORE INSERT ON sessions
			WHEN NEW.parked_at IS NOT NULL AND (NEW.state != 'stopped' OR NEW.archived_at IS NOT NULL)
			BEGIN
				SELECT RAISE(ABORT, 'session is in later');
			END;
-- +goose StatementEnd
-- apply declarative trigger "trg_sessions_later_update_guard" on table "sessions"
-- +goose StatementBegin
CREATE TRIGGER trg_sessions_later_update_guard
			BEFORE UPDATE OF state, archived_at, parked_at ON sessions
			WHEN NEW.parked_at IS NOT NULL AND (NEW.state != 'stopped' OR NEW.archived_at IS NOT NULL)
			BEGIN
				SELECT RAISE(ABORT, 'session is in later');
			END;
-- +goose StatementEnd
